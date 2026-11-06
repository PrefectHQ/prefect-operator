package controller

// Actions for the Kubernetes events the controllers emit. An event's reason
// says what happened, such as WorkPoolNotFound, and its action says what the
// controller was doing when it happened.
const (
	eventActionSync   = "Sync"
	eventActionDelete = "Delete"
)
