package inst

import (
	"time"

	"github.com/openark/golib/log"
	"github.com/openark/orchestrator/go/config"
)

type deadInstance struct {
	DelayFactor 	float32
	NextCheckTime   time.Time
}

var deadInstances = make(map[InstanceKey]deadInstance)

func RegisterDeadInstance(instanceKey *InstanceKey) {
	var maxDelay = time.Duration(config.Config.DeadInstancePollSecondsMax) * time.Second
	var exists bool
	var instance deadInstance
	var nextCheck time.Time
	currentIncreaseFactor := float32(1)

	now := time.Now()
	if _, exists = deadInstances[*instanceKey]; exists {
		currentIncreaseFactor = deadInstances[*instanceKey].DelayFactor
	}

	newIncreaseFactor := config.Config.DeadInstancePollSecondsMultiplyFactor * currentIncreaseFactor

	currentDelay := time.Duration(newIncreaseFactor * float32(config.Config.InstancePollSeconds)) * time.Second
	if currentDelay > maxDelay {
		currentDelay = maxDelay
		newIncreaseFactor = currentIncreaseFactor
	}
	nextCheck = now.Add(currentDelay)

	instance = deadInstance {
		DelayFactor: newIncreaseFactor,
		NextCheckTime: nextCheck,
	}
	deadInstances[*instanceKey] = instance

	log.Warningf("Instance %v:%v registered as dead. Current wait factor: %v (next: %v in %v secs)",
				instanceKey.Hostname, instanceKey.Port, instance.DelayFactor, instance.NextCheckTime, currentDelay)

}

func UnregisterDeadInstance(instanceKey *InstanceKey) {
	delete(deadInstances, *instanceKey)
}

func ShouldFilterOutDeadInstance(instanceKey *InstanceKey) bool {
	var exists bool
	if _, exists = deadInstances[*instanceKey]; !exists {
		return false
	}

	log.Debugf("Instance %v:%v, next check in: %v",
		instanceKey.Hostname, instanceKey.Port, deadInstances[*instanceKey].NextCheckTime.Sub(time.Now()))

	if deadInstances[*instanceKey].NextCheckTime.Before(time.Now()) {
		return false
	}

	log.Warningf("Instance %v:%v filtered out", instanceKey.Hostname, instanceKey.Port)
	return true
}