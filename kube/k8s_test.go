package kube

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/watch"
	"log"
	"testing"
)

func TestPVC(t *testing.T) {
	watcher, err := WatchPVC(context.Background(), "hello")
	if err != nil {
		panic(err)
	}
	resultChan := watcher.ResultChan()
	for event := range resultChan {
		pvc, ok := event.Object.(*v1.PersistentVolumeClaim)
		if !ok {
			log.Fatal("unexpected type")
		}

		maxClaims := "200Gi"
		var totalClaimedQuant resource.Quantity
		maxClaimedQuant := resource.MustParse(maxClaims)
		quant := pvc.Spec.Resources.Requests[v1.ResourceStorage]
		switch event.Type {
		case watch.Added:
			totalClaimedQuant.Add(quant)
			log.Printf("PVC %s added, claim size %s\n",
				pvc.Name, quant.String())
			if totalClaimedQuant.Cmp(maxClaimedQuant) == 1 {
				log.Printf(
					"\nClaim overage reached: max %s at %s",
					maxClaimedQuant.String(),
					totalClaimedQuant.String())
				// trigger action
				log.Println("*** Taking action ***")
			}
		case watch.Deleted:
			quant := pvc.Spec.Resources.Requests[v1.ResourceStorage]
			totalClaimedQuant.Sub(quant)
			log.Printf("PVC %s removed, size %s\n",
				pvc.Name, quant.String())
			if totalClaimedQuant.Cmp(maxClaimedQuant) <= 0 {
				log.Printf("Claim usage normal: max %s at %s",
					maxClaimedQuant.String(),
					totalClaimedQuant.String(),
				)
				// trigger action
				log.Println("*** Taking action ***")
			}
		}
	}
}
