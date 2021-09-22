package metrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"

	"github.com/iotaledger/hive.go/syncutils"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// MessageType defines the component for the different MPS metrics.
type MessageType byte

const (
	// DataMessage denotes data message type.
	DataMessage MessageType = iota
	// Transaction denotes transaction message.
	Transaction
)

// String returns the stringified component type.
func (c MessageType) String() string {
	switch c {
	case DataMessage:
		return "DataMessage"
	case Transaction:
		return "Transaction"
	default:
		return "Unknown"
	}
}

// ComponentType defines the component for the different MPS metrics.
type ComponentType byte

const (
	// Store denotes messages stored by the message store.
	Store ComponentType = iota
	// Solidifier denotes messages solidified by the solidifier.
	Solidifier
	// Scheduler denotes messages scheduled by the scheduler.
	Scheduler
	// Booker denotes messages booked by the booker.
	Booker
)

// String returns the stringified component type.
func (c ComponentType) String() string {
	switch c {
	case Store:
		return "Store"
	case Solidifier:
		return "Solidifier"
	case Scheduler:
		return "Scheduler"
	case Booker:
		return "Booker"
	default:
		return "Unknown"
	}
}

var (
	// Total number of processed messages since start of the node.
	messageTotalCount atomic.Uint64

	// number of messages in the database at startup
	initialMessageTotalCountDB uint64

	// total number of branches in the database at startup
	initialBranchTotalCountDB uint64

	// total number of finalized branches in the database at startup
	initialFinalizedBranchCountDB uint64

	// total number of confirmed branches in the database at startup
	initialConfirmedBranchCountDB uint64

	// number of branches created since the node started
	branchTotalCountDB atomic.Uint64

	// number of branches finalized since the node started
	finalizedBranchCountDB atomic.Uint64

	// current number of messages in the node's database
	messageTotalCountDB atomic.Uint64

	// number of solid messages in the database at startup
	initialMessageSolidCountDB uint64

	// current number of solid messages in the node's database
	messageSolidCountDBInc atomic.Uint64

	// helper variable that is only calculated at init phase. unit is milliseconds!
	initialSumSolidificationTime float64

	// sum of solidification time (since start of the node)
	sumSolidificationTime time.Duration
	solidTimeMutex        syncutils.RWMutex

	// initial number of missing messages in missingMessageStorage (at startup)
	initialMissingMessageCountDB uint64

	// current number of missing messages in missingMessageStorage
	missingMessageCountDB atomic.Uint64

	// current number of finalized messages
	finalizedMessageCount = make(map[MessageType]uint64)

	// protect map from concurrent read/write.
	finalizedMessageCountMutex syncutils.RWMutex

	// total time it took all messages to finalize. unit is milliseconds!
	messageFinalizationTotalTime = make(map[MessageType]uint64)

	// protect map from concurrent read/write.
	messageFinalizationTotalTimeMutex syncutils.RWMutex

	// current number of confirmed  branches
	confirmedBranchCount atomic.Uint64

	// total time it took all branches to finalize. unit is milliseconds!
	branchConfirmationTotalTime atomic.Uint64

	// current number of message tips.
	messageTips atomic.Uint64

	// total number of parents of all messages per parent type
	parentsCountPerType = make(map[tangle.ParentsType]uint64)

	// protect map from concurrent read/write.
	parentsCountPerTypeMutex syncutils.RWMutex

	// counter for the received MPS
	mpsReceivedSinceLastMeasurement atomic.Uint64

	// measured value of the received MPS
	measuredReceivedMPS atomic.Uint64

	// Number of messages per payload type since start of the node.
	messageCountPerPayload = make(map[payload.Type]uint64)

	// protect map from concurrent read/write.
	messageCountPerPayloadMutex syncutils.RWMutex

	// Number of messages per component (store, scheduler, booker) type since start of the node.
	// One for dashboard (reset every time is read), other for grafana with cumulative value.
	messageCountPerComponentDashboard = make(map[ComponentType]uint64)
	messageCountPerComponentGrafana   = make(map[ComponentType]uint64)

	// protect map from concurrent read/write.
	messageCountPerComponentMutex syncutils.RWMutex

	// number of messages being requested by the message layer.
	requestQueueSize atomic.Int64
)

////// Exported functions to obtain metrics from outside //////

// MessageTotalCountSinceStart returns the total number of messages seen since the start of the node.
func MessageTotalCountSinceStart() uint64 {
	return messageTotalCount.Load()
}

// MessageCountSinceStartPerPayload returns a map of message payload types and their count since the start of the node.
func MessageCountSinceStartPerPayload() map[payload.Type]uint64 {
	messageCountPerPayloadMutex.RLock()
	defer messageCountPerPayloadMutex.RUnlock()

	// copy the original map
	clone := make(map[payload.Type]uint64)
	for key, element := range messageCountPerPayload {
		clone[key] = element
	}

	return clone
}

// MessageCountSinceStartPerComponentGrafana returns a map of message count per component types and their count since the start of the node.
func MessageCountSinceStartPerComponentGrafana() map[ComponentType]uint64 {
	messageCountPerComponentMutex.RLock()
	defer messageCountPerComponentMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]uint64)
	for key, element := range messageCountPerComponentGrafana {
		clone[key] = element
	}

	return clone
}

// MessageCountSinceStartPerComponentDashboard returns a map of message count per component types and their count since last time the value was read.
func MessageCountSinceStartPerComponentDashboard() map[ComponentType]uint64 {
	messageCountPerComponentMutex.RLock()
	defer messageCountPerComponentMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]uint64)
	for key, element := range messageCountPerComponentDashboard {
		clone[key] = element
	}

	return clone
}

// MessageTips returns the actual number of tips in the message tangle.
func MessageTips() uint64 {
	return messageTips.Load()
}

// MessageRequestQueueSize returns the number of message requests the node currently has registered.
func MessageRequestQueueSize() int64 {
	return requestQueueSize.Load()
}

// TotalBranchCountDB returns the total number of branches.
func TotalBranchCountDB() uint64 {
	return initialBranchTotalCountDB + branchTotalCountDB.Load()
}

// FinalizedBranchCountDB returns the number of non-confirmed branches.
func FinalizedBranchCountDB() uint64 {
	return initialFinalizedBranchCountDB + finalizedBranchCountDB.Load()
}

// MessageSolidCountDB returns the number of messages that are solid in the DB.
func MessageSolidCountDB() uint64 {
	return initialMessageSolidCountDB + messageSolidCountDBInc.Load()
}

// MessageTotalCountDB returns the number of messages that are stored in the DB.
func MessageTotalCountDB() uint64 {
	return initialMessageTotalCountDB + messageTotalCountDB.Load()
}

// AvgSolidificationTime returns the average time it takes for a message to become solid. [milliseconds]
func AvgSolidificationTime() (result float64) {
	solidTimeMutex.RLock()
	defer solidTimeMutex.RUnlock()
	totalSolid := MessageSolidCountDB()
	if totalSolid > 0 {
		result = (initialSumSolidificationTime + float64(sumSolidificationTime.Milliseconds())) / float64(totalSolid)
	}
	return
}

// MessageMissingCountDB returns the number of messages in missingMessageStore.
func MessageMissingCountDB() uint64 {
	return initialMissingMessageCountDB + missingMessageCountDB.Load()
}

// MessageFinalizationTotalTimePerType returns total time it took for all messages to finalize per message type.
func MessageFinalizationTotalTimePerType() map[MessageType]uint64 {
	messageFinalizationTotalTimeMutex.RLock()
	defer messageFinalizationTotalTimeMutex.RUnlock()

	// copy the original map
	clone := make(map[MessageType]uint64)
	for key, element := range messageFinalizationTotalTime {
		clone[key] = element
	}

	return clone
}

// FinalizedMessageCountPerType returns the number of messages finalized per message type.
func FinalizedMessageCountPerType() map[MessageType]uint64 {
	finalizedMessageCountMutex.RLock()
	defer finalizedMessageCountMutex.RUnlock()

	// copy the original map
	clone := make(map[MessageType]uint64)
	for key, element := range finalizedMessageCount {
		clone[key] = element
	}

	return clone
}

// BranchConfirmationTotalTime returns total time it took for all confirmed branches to be confirmed.
func BranchConfirmationTotalTime() uint64 {
	return branchConfirmationTotalTime.Load()
}

// ConfirmedBranchCount returns the number of confirmed branches.
func ConfirmedBranchCount() uint64 {
	return initialConfirmedBranchCountDB + confirmedBranchCount.Load()
}

// ParentCountPerType returns a map of parent counts per parent type.
func ParentCountPerType() map[tangle.ParentsType]uint64 {
	parentsCountPerTypeMutex.RLock()
	defer parentsCountPerTypeMutex.RUnlock()

	// copy the original map
	clone := make(map[tangle.ParentsType]uint64)
	for key, element := range parentsCountPerType {
		clone[key] = element
	}

	return clone
}

// ReceivedMessagesPerSecond retrieves the current messages per second number.
func ReceivedMessagesPerSecond() uint64 {
	return measuredReceivedMPS.Load()
}

////// Handling data updates and measuring //////

func increasePerPayloadCounter(p payload.Type) {
	messageCountPerPayloadMutex.Lock()
	defer messageCountPerPayloadMutex.Unlock()

	// increase cumulative metrics
	messageCountPerPayload[p]++
	messageTotalCount.Inc()
}

func increasePerComponentCounter(c ComponentType) {
	messageCountPerComponentMutex.Lock()
	defer messageCountPerComponentMutex.Unlock()

	// increase cumulative metrics
	messageCountPerComponentDashboard[c]++
	messageCountPerComponentGrafana[c]++
}

func increasePerParentType(c tangle.ParentsType) {
	parentsCountPerTypeMutex.Lock()
	defer parentsCountPerTypeMutex.Unlock()

	// increase cumulative metrics
	parentsCountPerType[c]++
}

// measures the Component Counter value per second
func measurePerComponentCounter() {
	// sample the current counter value into a measured MPS value
	componentCounters := MessageCountSinceStartPerComponentDashboard()

	// reset the counter
	messageCountPerComponentMutex.Lock()
	for key := range messageCountPerComponentDashboard {
		messageCountPerComponentDashboard[key] = 0
	}
	messageCountPerComponentMutex.Unlock()

	// trigger events for outside listeners
	Events.ComponentCounterUpdated.Trigger(componentCounters)
}

func measureMessageTips() {
	metrics.Events().MessageTips.Trigger(uint64(messagelayer.Tangle().TipManager.TipCount()))
}

// increases the received MPS counter
func increaseReceivedMPSCounter() {
	mpsReceivedSinceLastMeasurement.Inc()
}

// measures the received MPS value
func measureReceivedMPS() {
	// sample the current counter value into a measured MPS value
	sampledMPS := mpsReceivedSinceLastMeasurement.Load()

	// store the measured value
	measuredReceivedMPS.Store(sampledMPS)

	// reset the counter
	mpsReceivedSinceLastMeasurement.Store(0)

	// trigger events for outside listeners
	Events.ReceivedMPSUpdated.Trigger(sampledMPS)
}

func measureRequestQueueSize() {
	size := int64(messagelayer.Tangle().Requester.RequestQueueSize())
	requestQueueSize.Store(size)
}

func measureInitialDBStats() {
	solid, total, avgSolidTime, missing := messagelayer.Tangle().Storage.DBStats()
	initialMessageSolidCountDB = uint64(solid)
	initialMessageTotalCountDB = uint64(total)
	initialSumSolidificationTime = avgSolidTime * float64(solid)
	initialMissingMessageCountDB = uint64(missing)

	messagelayer.Tangle().LedgerState.BranchDAG.ForEachBranch(func(branch ledgerstate.Branch) {
		switch branch.ID() {
		case ledgerstate.MasterBranchID:
			return
		case ledgerstate.InvalidBranchID:
			return
		case ledgerstate.LazyBookedConflictsBranchID:
			return
		default:
			initialBranchTotalCountDB++
			branchGoF, err := messagelayer.Tangle().LedgerState.UTXODAG.BranchGradeOfFinality(branch.ID())
			if err != nil {
				return
			}
			if branchGoF == gof.High {
				messagelayer.Tangle().LedgerState.BranchDAG.ForEachConflictingBranchID(branch.ID(), func(conflictingBranchID ledgerstate.BranchID) {
					if conflictingBranchID != branch.ID() {
						initialFinalizedBranchCountDB++
					}
				})
				initialFinalizedBranchCountDB++
				initialConfirmedBranchCountDB++
			}
		}
	})
}
