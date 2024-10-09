package node

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/hashicorp/raft"
)

// Get value by key
func (s *RStorage) Get(key string) string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.storage[key]
}

// Set value by key
func (s *RStorage) Set(key string, value string) error {
	if s.RaftNode.State() != raft.Leader {
		return fmt.Errorf("Only leader can write to the storage")
	}

	event := &logEvent{
		Type:  "set",
		Key:   key,
		Value: value,
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	timeout := time.Second * 5
	_ = s.RaftNode.Apply(data, timeout)
	return nil
}

type logEvent struct {
	Type  string
	Key   string
	Value string
}

// Apply applies a Raft log entry to the key-value store.
func (s *RStorage) Apply(logEntry *raft.Log) interface{} {
	s.logger.Debug("Applying a new log entry to the store")
	var event logEvent
	if err := json.Unmarshal(logEntry.Data, &event); err != nil {
		s.logger.Error("Can't read Raft log event", "error", err)
		return nil
	}
	if event.Type == "set" {
		s.logger.Debug("Set operation received", "key", event.Key, "value", event.Value)
		s.mutex.Lock()
		defer s.mutex.Unlock()
		s.storage[event.Key] = event.Value
		return nil
	}
	s.logger.Warn("Unknown Raft log event type", "type", event.Type)
	return nil
}

// fsmSnapshot is used by Raft library to save a point-in-time snapshot of the FSM
type fsmSnapshot struct {
	storage map[string]string
}

// Snapshot returns FSMSnapshot which is used to save snapshot of the FSM
func (s *RStorage) Snapshot() (raft.FSMSnapshot, error) {
	s.logger.Debug("Creating snapshot")
	s.mutex.Lock()
	defer s.mutex.Unlock()
	storageCopy := make(map[string]string)
	for k, v := range s.storage {
		storageCopy[k] = v
	}
	return &fsmSnapshot{storage: storageCopy}, nil
}

// Restore stores the key-value store to a previous state.
func (s *RStorage) Restore(serialized io.ReadCloser) error {
	s.logger.Debug("Restoring from snapshot")
	var snapshot fsmSnapshot
	if err := json.NewDecoder(serialized).Decode(&snapshot); err != nil {
		return err
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.storage = snapshot.storage
	return nil
}

// Persist should dump all necessary state to the WriteCloser 'sink',
// and call sink.Close() when finished or call sink.Cancel() on error.
func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	snapshotBytes, err := json.Marshal(f)
	if err != nil {
		sink.Cancel()
		return err
	}
	if _, err := sink.Write(snapshotBytes); err != nil {
		sink.Cancel()
		return err
	}
	return sink.Close()
}

// Release is invoked when the Raft library is finished with the snapshot.
func (f *fsmSnapshot) Release() {}
