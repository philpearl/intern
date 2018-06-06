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
	table          []entry
	oldTable       []entry
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
		table: make([]entry, cap),
	}
}

// Len returns the number of unique strings stored
func (i *Intern) Len() int {
	return i.count
}

// Cap returns the size of the intern table
func (i *Intern) Cap() int {
	return len(i.table)
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

	if i.oldTable != nil {
		_, index := i.findInTable(i.table, val, hash)
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
	i.table[cursor] = entry{
		hash:  hash,
		index: int32(offset + 1),
	}
	i.count++

	return offset
}

// findInTable find the string val in the hash table. If the string is present, it returns the
// place in the table where it was found, plus the stringbank offset of the string + 1
func (i *Intern) findInTable(table []entry, val string, hashVal uint32) (cursor int, index int) {
	l := len(table)
	cursor = int(hashVal) & (l - 1)
	start := cursor
	for table[cursor].index != 0 {
		e := &table[cursor]
		if e.hash == hashVal && i.sb.Get(int(e.index-1)) == val {
			return cursor, int(e.index)
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

func (i *Intern) copyEntryToTable(table []entry, e entry) {
	l := len(table)
	cursor := int(e.hash) & (l - 1)
	start := cursor
	for table[cursor].index != 0 {
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
	table[cursor] = e
}

func (i *Intern) resize() {
	if i.table == nil {
		i.table = make([]entry, 16)
	}

	if i.count < len(i.table)*3/4 && i.oldTable == nil {
		return
	}

	if i.oldTable == nil {
		i.oldTable, i.table = i.table, make([]entry, len(i.table)*2)
	}

	// We copy items between tables 16 at a time. Since we do this every time
	// anyone writes to the table we won't run out of space in the new table
	// before this is complete
	l := len(i.oldTable)
	for k := 0; k < 16; k++ {
		e := i.oldTable[k+i.oldTableCursor]
		if e.index != 0 {
			i.copyEntryToTable(i.table, e)
			// The entry can exist in the old and new versions of the table without
			// problems. If we did try to delete from the old table we'd have issues
			// searching forward from clashing entries.
		}
	}
	i.oldTableCursor += 16
	if i.oldTableCursor >= l {
		i.oldTable = nil
		i.oldTableCursor = 0
	}
}

type entry struct {
	// We keep the hash alongside each entry to make it much faster to resize
	// It also speeds up stepping through entries when hashes clash
	hash uint32
	// index is the index of the string in the stringbank, plus 1 so that valid
	// entries are never zero
	index int32
}
