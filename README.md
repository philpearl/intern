
# A string interning library for Go

[![GoDoc](https://godoc.org/github.com/philpearl/intern?status.svg)](https://godoc.org/github.com/philpearl/intern)

intern has a number of benefits

1. It deduplicates strings. if you have data that references identical text in very many different places if may significantly reduce the amount of memory used for the strings.
2. It stores the strings in a way that reduces the load on the garbage collector.
3. It allows you to store an int ID for the string instead of the string itself. This is considerably smaller, and again is GC friendly.

```go
i := intern.New()
hat := i.Save("hat")
fmt.Printf(i.Get(hat))
```