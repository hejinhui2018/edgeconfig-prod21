package persistence

import "encoding/json"

func cloneSnapshot(snapshot Snapshot) (Snapshot, error) {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return Snapshot{}, err
	}
	var copy Snapshot
	if err := json.Unmarshal(data, &copy); err != nil {
		return Snapshot{}, err
	}
	if err := copy.Normalize(); err != nil {
		return Snapshot{}, err
	}
	return copy, nil
}
