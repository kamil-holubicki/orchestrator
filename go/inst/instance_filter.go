package inst

import (
	"sync"
	"time"

	"github.com/openark/golib/log"
	"github.com/openark/orchestrator/go/config"
	"github.com/rcrowley/go-metrics"
)

type deadInstance struct {
	DelayFactor 	float32
	NextCheckTime   time.Time
}

var deadInstances = make(map[InstanceKey]deadInstance)
var deadInstancesMutex = sync.RWMutex{}
var deadInstancesCounter = metrics.NewCounter()

func DeadInstanceFilterInit() {
	metrics.Register("discoveries.dead_instances", deadInstancesCounter)
}

func RegisterDeadInstance(instanceKey *InstanceKey) {
	// Dead instance discovery delaying mechanism is disabled when we have no
	// exponential backoff.
	if config.Config.DeadInstancePollSecondsMultiplyFactor == 1 {
		return
	}

	currentIncreaseFactor := float32(1)

	deadInstancesMutex.Lock()
	defer deadInstancesMutex.Unlock()

	instance, exists := deadInstances[*instanceKey]
	if exists {
		currentIncreaseFactor = instance.DelayFactor
	} else {
		deadInstancesCounter.Inc(1)
	}

	newIncreaseFactor := config.Config.DeadInstancePollSecondsMultiplyFactor * currentIncreaseFactor

	maxDelay := time.Duration(config.Config.DeadInstancePollSecondsMax) * time.Second
	currentDelay := time.Duration(newIncreaseFactor * float32(config.Config.InstancePollSeconds)) * time.Second
	if currentDelay > maxDelay {
		currentDelay = maxDelay
		newIncreaseFactor = currentIncreaseFactor
	}
	nextCheck := time.Now().Add(currentDelay)

	instance = deadInstance {
		DelayFactor: newIncreaseFactor,
		NextCheckTime: nextCheck,
	}
	deadInstances[*instanceKey] = instance

	log.Debugf("Instance %v:%v registered as dead. Current wait factor: %v (next: %v in %v secs)",
				instanceKey.Hostname, instanceKey.Port, instance.DelayFactor, instance.NextCheckTime, currentDelay)
}

func UnregisterDeadInstance(instanceKey *InstanceKey) {
	deadInstancesMutex.Lock()
	defer deadInstancesMutex.Unlock()

	_, exists := deadInstances[*instanceKey]
	if exists {
		deadInstancesCounter.Dec(1)
	}
	delete(deadInstances, *instanceKey)
}

func DeadInstanceRecheckNeeded(instanceKey *InstanceKey) (bool, bool) {
	deadInstancesMutex.RLock()
	defer deadInstancesMutex.RUnlock()

	instance, exists := deadInstances[*instanceKey]

	if !exists {
		return false, false
	}

	log.Debugf("DeadInstanceRecheckNeeded: %v:%v, next check in: %v",
		instanceKey.Hostname, instanceKey.Port, time.Until(instance.NextCheckTime))

	if instance.NextCheckTime.After(time.Now()) {
		return true, false
	}

	log.Debugf("DeadInstanceRecheckNeeded: %v:%v - recheck", instanceKey.Hostname, instanceKey.Port)
	return true, true
}