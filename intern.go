// Package intern is a string interning library. Pass it a string, and it will store it and return it, removing duplicates. That is, however many times you show it a string, it will only store that string once, and will always
// return a version of it backed by the same memory.
//
// Storage is kind to GC. It is optimised for storing a very large number of strings.
package intern

import (
	"math/bits"

	"github.com/philpearl/aeshash"
	"github.com/philpearl/stringbank"
)

// Intern implements the interner. Allocate it
type Intern struct {
	sb             stringbank.Stringbank
	table          table
	oldTable       table
	count          int
	oldTableCursor int
}

// New creates a new interning table
func New(cap int) *Intern {
	if cap < 16 {
		cap = 16
	} else {
		cap = 1 << uint(64-bits.LeadingZeros(uint(cap-1)))
	}
	return &Intern{
		table: table{
			hashes:  make([]uint32, cap),
			indices: make([]int32, cap),
		},
	}
}

// Len returns the number of unique strings stored
func (i *Intern) Len() int {
	return i.count
}

// Cap returns the size of the intern table
func (i *Intern) Cap() int {
	return i.table.len()
}

// Get returns the stored string for an offset. Offset can be obtained via OffsetFor.
func (i *Intern) Get(offset int) string {
	return i.sb.Get(offset)
}

// Deduplicate takes a string and returns a permanently stored version. This will always
// be backed by the same memory for the same string.
func (i *Intern) Deduplicate(val string) string {
	return i.Get(i.OffsetFor(val))
}

// OffsetFor returns an integer offset for the requested string in our deduplicated string store
func (i *Intern) OffsetFor(val string) int {
	// we use a hashtable where the keys are stringbank offsets, but comparisons are done on
	// strings. There is no value to store
	i.resize()

	hash := aeshash.Hash(val)

	if i.oldTable.len() != 0 {
		_, index := i.findInTable(i.oldTable, val, hash)
		if index != 0 {
			return index - 1
		}
	}

	cursor, index := i.findInTable(i.table, val, hash)
	if index != 0 {
		return index - 1
	}

	// String was not found, so we want to store it. Cursor is the index where we should
	// store it
	offset := i.sb.Save(val)
	i.table.hashes[cursor] = hash
	i.table.indices[cursor] = int32(offset + 1)
	i.count++

	return offset
}

// findInTable find the string val in the hash table. If the string is present, it returns the
// place in the table where it was found, plus the stringbank offset of the string + 1
func (i *Intern) findInTable(table table, val string, hashVal uint32) (cursor int, index int) {
	l := table.len()
	cursor = int(hashVal) & (l - 1)
	start := cursor
	for table.indices[cursor] != 0 {
		if table.hashes[cursor] == hashVal {
			if index := int(table.indices[cursor]); i.sb.Get(index-1) == val {
				return cursor, index
			}
		}
		cursor++
		if cursor == l {
			cursor = 0
		}
		if cursor == start {
			panic("out of space!")
		}
	}
	return cursor, 0
}

func (i *Intern) copyEntryToTable(table table, index int32, hash uint32) {
	l := table.len()
	cursor := int(hash) & (l - 1)
	start := cursor
	for table.indices[cursor] != 0 {
		// the entry we're copying in is guaranteed not to be already
		// present, so we're just looking for an empty space
		cursor++
		if cursor == l {
			cursor = 0
		}
		if cursor == start {
			panic("out of space (resize)!")
		}
	}
	table.indices[cursor] = index
	table.hashes[cursor] = hash
}

func (i *Intern) resize() {
	if i.table.hashes == nil {
		i.table.hashes = make([]uint32, 16)
		i.table.indices = make([]int32, 16)
	}

	if i.count < i.table.len()*3/4 && i.oldTable.len() == 0 {
		return
	}

	if i.oldTable.hashes == nil {
		i.oldTable, i.table = i.table, table{
			hashes:  make([]uint32, len(i.table.hashes)*2),
			indices: make([]int32, len(i.table.indices)*2),
		}
	}

	// We copy items between tables 16 at a time. Since we do this every time
	// anyone writes to the table we won't run out of space in the new table
	// before this is complete
	l := i.oldTable.len()
	for k := 0; k < 16; k++ {
		if index := i.oldTable.indices[k+i.oldTableCursor]; index != 0 {
			i.copyEntryToTable(i.table, index, i.oldTable.hashes[k+i.oldTableCursor])
			// The entry can exist in the old and new versions of the table without
			// problems. If we did try to delete from the old table we'd have issues
			// searching forward from clashing entries.
		}
	}
	i.oldTableCursor += 16
	if i.oldTableCursor >= l {
		i.oldTable.hashes = nil
		i.oldTable.indices = nil
		i.oldTableCursor = 0
	}
}

// table represents a hash table. We keep the indices and hashes separate in
// case we want to use different size types in the future
type table struct {
	// We keep hashes in the table to speed up resizing, and also stepping through
	// entries that have different hashes but hit the same bucket
	hashes []uint32
	// index is the index of the string in the stringbank, plus 1 so that valid
	// entries are never zero
	indices []int32
}

func (t table) len() int {
	return len(t.hashes)
}
