// Copyright 2014 The Cayley Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package writer

import (
	"sync"
	"time"

	"github.com/google/cayley/graph"
	"github.com/google/cayley/quad"
)

func init() {
	graph.RegisterWriter("single", NewSingleReplication)
}

type Single struct {
	nextID int64
	qs     graph.QuadStore
	mut    sync.Mutex
}

func NewSingleReplication(qs graph.QuadStore, opts graph.Options) (graph.QuadWriter, error) {
	horizon := qs.Horizon()
	rep := &Single{nextID: horizon + 1, qs: qs}
	if horizon <= 0 {
		rep.nextID = 1
	}
	return rep, nil
}

func (s *Single) AcquireNextID() int64 {
	s.mut.Lock()
	defer s.mut.Unlock()
	id := s.nextID
	s.nextID++
	return id
}

func (s *Single) AddQuad(q quad.Quad) error {
	deltas := make([]graph.Delta, 1)
	deltas[0] = graph.Delta{
		ID:        s.AcquireNextID(),
		Quad:      q,
		Action:    graph.Add,
		Timestamp: time.Now(),
	}
	return s.qs.ApplyDeltas(deltas)
}

func (s *Single) AddQuadSet(set []quad.Quad) error {
	deltas := make([]graph.Delta, len(set))
	for i, q := range set {
		deltas[i] = graph.Delta{
			ID:        s.AcquireNextID(),
			Quad:      q,
			Action:    graph.Add,
			Timestamp: time.Now(),
		}
	}
	s.qs.ApplyDeltas(deltas)
	return nil
}

func (s *Single) RemoveQuad(q quad.Quad) error {
	deltas := make([]graph.Delta, 1)
	deltas[0] = graph.Delta{
		ID:        s.AcquireNextID(),
		Quad:      q,
		Action:    graph.Delete,
		Timestamp: time.Now(),
	}
	return s.qs.ApplyDeltas(deltas)
}

func (s *Single) Close() error {
	// Nothing to clean up locally.
	return nil
}
