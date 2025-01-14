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
	TryCnt			int
}

type deadInstancesFilter struct {
	deadInstances 		map[InstanceKey]deadInstance
	deadInstancesMutex 	sync.RWMutex
}

var DeadInstancesFilter deadInstancesFilter

var deadInstancesCounter = metrics.NewCounter()

func init() {
	metrics.Register("discoveries.dead_instances", deadInstancesCounter)
	DeadInstancesFilter.deadInstances = make(map[InstanceKey]deadInstance)
	DeadInstancesFilter.deadInstancesMutex = sync.RWMutex{}
}

func (f *deadInstancesFilter) RegisterInstance(instanceKey *InstanceKey) {
	// The behavior depends on settings:
	// 1. DeadInstanceDiscoveryMaxConcurrency > 0 and DeadInstancePollSecondsMultiplyFactor > 1:
	//    The separate discovery queue for dead instances is created and dead instances
	// 	  are checked by dedicated pool of go workers
	//    and the instance is checked with exponential backoff mechanism time
	// 2. DeadInstanceDiscoveryMaxConcurrency = 0 and DeadInstancePollSecondsMultiplyFactor > 1:
	//    No separate discovery queue for dead instances is created and dead instances
	//    are checked by the same pool of go workers as healthy instances, however
	//    an exponential backoff mechanism is applied for dead instances
	// 3. DeadInstanceDiscoveryMaxConcurrency > 0 and DeadInstancePollSecondsMultiplyFactor = 1:
	//    The separate discovery queue for dead instances is created and dead instances
	//    are checked by dedicated pool of go workers. No exponential backoff mechanism
	//    is applied for dead instances
	// 4. DeadInstanceDiscoveryMaxConcurrency = 0 and DeadInstancePollSecondsMultiplyFactor = 1:
	//    No separate discovery queue for dead instances, no dedicated go workers,
	//    no backoff mechanism. This is the default working mode.
	//
	// We register a dead instance always. It shouldn't be a big overhead,
	// and we will get the info about the dead instances count.
	currentIncreaseFactor := float32(1)
	previousTry := 0

	f.deadInstancesMutex.Lock()
	defer f.deadInstancesMutex.Unlock()

	instance, exists := f.deadInstances[*instanceKey]
	if exists {
		currentIncreaseFactor = instance.DelayFactor
		previousTry = instance.TryCnt
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
		TryCnt: previousTry + 1,
	}
	f.deadInstances[*instanceKey] = instance

	if config.Config.DeadInstancesDiscoveryLogsEnabled {
		log.Debugf("Dead instance registered %v:%v. Iteration: %v. Current wait factor: %v (next check in %v secs (on %v))",
			instanceKey.Hostname, instanceKey.Port, instance.TryCnt, instance.DelayFactor, currentDelay, instance.NextCheckTime)
	}
}

func (f *deadInstancesFilter)UnregisterInstance(instanceKey *InstanceKey) {
	f.deadInstancesMutex.Lock()
	defer f.deadInstancesMutex.Unlock()

	instance, exists := f.deadInstances[*instanceKey]
	if exists {
		if config.Config.DeadInstancesDiscoveryLogsEnabled {
			log.Debugf("Dead instance unregistered: %v:%v after iteration: %v",
				instanceKey.Hostname, instanceKey.Port, instance.TryCnt)
		}
		deadInstancesCounter.Dec(1)
		delete(f.deadInstances, *instanceKey)
	}
}

func (f *deadInstancesFilter)InstanceRecheckNeeded(instanceKey *InstanceKey) (bool, bool) {
	f.deadInstancesMutex.RLock()
	defer f.deadInstancesMutex.RUnlock()

	instance, exists := f.deadInstances[*instanceKey]

	if !exists {
		return false, false
	}

	if instance.NextCheckTime.After(time.Now()) {
		return true, false
	}

	if config.Config.DeadInstancesDiscoveryLogsEnabled {
		log.Debugf("Dead instance recheck: %v:%v. Iteration: %v",
			instanceKey.Hostname, instanceKey.Port, instance.TryCnt)
	}
	return true, true
}