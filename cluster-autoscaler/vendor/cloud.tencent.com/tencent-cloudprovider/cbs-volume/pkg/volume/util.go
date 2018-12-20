package volume

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"strings"
	"strconv"
	"hash/fnv"
	"math/rand"
	"fmt"
	"github.com/golang/glog"
)

func RoundUpSize(volumeSizeBytes int64, allocationUnitBytes int64) int64 {
	return (volumeSizeBytes + allocationUnitBytes - 1) / allocationUnitBytes
}

func ZonesToSet(zonesString string) (sets.String, error) {
	zonesSlice := strings.Split(zonesString, ",")
	zonesSet := make(sets.String)
	for _, zone := range zonesSlice {
		trimmedZone := strings.TrimSpace(zone)
		if trimmedZone == "" {
			return make(sets.String), fmt.Errorf("comma separated list of zones (%q) must not contain an empty zone", zonesString)
		}
		zonesSet.Insert(trimmedZone)
	}
	return zonesSet, nil
}


// ChooseZone implements our heuristics for choosing a zone for volume creation based on the volume name
// Volumes are generally round-robin-ed across all active zones, using the hash of the PVC Name.
// However, if the PVCName ends with `-<integer>`, we will hash the prefix, and then add the integer to the hash.
// This means that a StatefulSet's volumes (`claimname-statefulsetname-id`) will spread across available zones,
// assuming the id values are consecutive.
func ChooseZoneForVolume(zones sets.String, pvcName string) string {
	// We create the volume in a zone determined by the name
	// Eventually the scheduler will coordinate placement into an available zone
	var hash uint32
	var index uint32

	if pvcName == "" {
		// We should always be called with a name; this shouldn't happen
		glog.Warningf("No name defined during volume create; choosing random zone")

		hash = rand.Uint32()
	} else {
		hashString := pvcName

		// Heuristic to make sure that volumes in a StatefulSet are spread across zones
		// StatefulSet PVCs are (currently) named ClaimName-StatefulSetName-Id,
		// where Id is an integer index.
		// Note though that if a StatefulSet pod has multiple claims, we need them to be
		// in the same zone, because otherwise the pod will be unable to mount both volumes,
		// and will be unschedulable.  So we hash _only_ the "StatefulSetName" portion when
		// it looks like `ClaimName-StatefulSetName-Id`.
		// We continue to round-robin volume names that look like `Name-Id` also; this is a useful
		// feature for users that are creating statefulset-like functionality without using statefulsets.
		lastDash := strings.LastIndexByte(pvcName, '-')
		if lastDash != -1 {
			statefulsetIDString := pvcName[lastDash + 1:]
			statefulsetID, err := strconv.ParseUint(statefulsetIDString, 10, 32)
			if err == nil {
				// Offset by the statefulsetID, so we round-robin across zones
				index = uint32(statefulsetID)
				// We still hash the volume name, but only the prefix
				hashString = pvcName[:lastDash]

				// In the special case where it looks like `ClaimName-StatefulSetName-Id`,
				// hash only the StatefulSetName, so that different claims on the same StatefulSet
				// member end up in the same zone.
				// Note that StatefulSetName (and ClaimName) might themselves both have dashes.
				// We actually just take the portion after the final - of ClaimName-StatefulSetName.
				// For our purposes it doesn't much matter (just suboptimal spreading).
				lastDash := strings.LastIndexByte(hashString, '-')
				if lastDash != -1 {
					hashString = hashString[lastDash + 1:]
				}

				glog.V(2).Infof("Detected StatefulSet-style volume name %q; index=%d", pvcName, index)
			}
		}

		// We hash the (base) volume name, so we don't bias towards the first N zones
		h := fnv.New32()
		h.Write([]byte(hashString))
		hash = h.Sum32()
	}

	// Zones.List returns zones in a consistent order (sorted)
	// We do have a potential failure case where volumes will not be properly spread,
	// if the set of zones changes during StatefulSet volume creation.  However, this is
	// probably relatively unlikely because we expect the set of zones to be essentially
	// static for clusters.
	// Hopefully we can address this problem if/when we do full scheduler integration of
	// PVC placement (which could also e.g. avoid putting volumes in overloaded or
	// unhealthy zones)
	zoneSlice := zones.List()
	zone := zoneSlice[(hash + index) % uint32(len(zoneSlice))]

	glog.V(2).Infof("Creating volume for PVC %q; chose zone=%q from zones=%q", pvcName, zone, zoneSlice)
	return zone
}
