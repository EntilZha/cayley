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

package bolt

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/barakmich/glog"
	"github.com/boltdb/bolt"

	"github.com/google/cayley/graph"
	"github.com/google/cayley/graph/iterator"
	"github.com/google/cayley/quad"
)

type AllIterator struct {
	uid    uint64
	tags   graph.Tagger
	bucket []byte
	dir    quad.Direction
	qs     *QuadStore
	result *Token
	buffer [][]byte
	offset int
	done   bool
}

func NewAllIterator(bucket []byte, d quad.Direction, qs *QuadStore) *AllIterator {
	return &AllIterator{
		uid:    iterator.NextUID(),
		bucket: bucket,
		dir:    d,
		qs:     qs,
	}
}

func (it *AllIterator) UID() uint64 {
	return it.uid
}

func (it *AllIterator) Reset() {
	it.buffer = nil
	it.offset = 0
	it.done = false
}

func (it *AllIterator) Tagger() *graph.Tagger {
	return &it.tags
}

func (it *AllIterator) TagResults(dst map[string]graph.Value) {
	for _, tag := range it.tags.Tags() {
		dst[tag] = it.Result()
	}

	for tag, value := range it.tags.Fixed() {
		dst[tag] = value
	}
}

func (it *AllIterator) Clone() graph.Iterator {
	out := NewAllIterator(it.bucket, it.dir, it.qs)
	out.tags.CopyFrom(it)
	return out
}

func (it *AllIterator) Next() bool {
	if it.done {
		return false
	}
	if len(it.buffer) <= it.offset+1 {
		it.offset = 0
		var last []byte
		if it.buffer != nil {
			last = it.buffer[len(it.buffer)-1]
		}
		it.buffer = make([][]byte, 0, bufferSize)
		err := it.qs.db.View(func(tx *bolt.Tx) error {
			i := 0
			b := tx.Bucket(it.bucket)
			cur := b.Cursor()
			if last == nil {
				k, _ := cur.First()
				var out []byte
				out = make([]byte, len(k))
				copy(out, k)
				it.buffer = append(it.buffer, out)
				i++
			} else {
				k, _ := cur.Seek(last)
				if !bytes.Equal(k, last) {
					return fmt.Errorf("could not pick up after %v", k)
				}
			}
			for i < bufferSize {
				k, _ := cur.Next()
				if k == nil {
					it.buffer = append(it.buffer, k)
					break
				}
				var out []byte
				out = make([]byte, len(k))
				copy(out, k)
				it.buffer = append(it.buffer, out)
				i++
			}
			return nil
		})
		if err != nil {
			glog.Error("Error nexting in database: ", err)
			it.done = true
			return false
		}
	} else {
		it.offset++
	}
	if it.Result() == nil {
		it.done = true
		return false
	}
	return true
}

func (it *AllIterator) ResultTree() *graph.ResultTree {
	return graph.NewResultTree(it.Result())
}

func (it *AllIterator) Result() graph.Value {
	if it.done {
		return nil
	}
	if it.result != nil {
		return it.result
	}
	if it.offset >= len(it.buffer) {
		return nil
	}
	if it.buffer[it.offset] == nil {
		return nil
	}
	return &Token{bucket: it.bucket, key: it.buffer[it.offset]}
}

func (it *AllIterator) NextPath() bool {
	return false
}

// No subiterators.
func (it *AllIterator) SubIterators() []graph.Iterator {
	return nil
}

func (it *AllIterator) Contains(v graph.Value) bool {
	it.result = v.(*Token)
	return true
}

func (it *AllIterator) Close() {
	it.result = nil
	it.buffer = nil
	it.done = true
}

func (it *AllIterator) Size() (int64, bool) {
	return it.qs.size, true
}

func (it *AllIterator) DebugString(indent int) string {
	size, _ := it.Size()
	return fmt.Sprintf("%s(%s tags: %v bolt size:%d %s %p)", strings.Repeat(" ", indent), it.Type(), it.tags.Tags(), size, it.dir, it)
}

func (it *AllIterator) Type() graph.Type { return graph.All }
func (it *AllIterator) Sorted() bool     { return false }

func (it *AllIterator) Optimize() (graph.Iterator, bool) {
	return it, false
}

func (it *AllIterator) Stats() graph.IteratorStats {
	s, _ := it.Size()
	return graph.IteratorStats{
		ContainsCost: 1,
		NextCost:     2,
		Size:         s,
	}
}
