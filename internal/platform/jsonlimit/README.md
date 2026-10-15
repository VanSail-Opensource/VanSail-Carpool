# Bounded JSON decoding

`reader.go` provides the streaming Reader checks nesting, array length, token count and encoded string
length before encoding/json builds typed values. It never replaces syntax
validation. Limits are caller supplied: scanners bound entire response graphs,
while streaming inventories permit arbitrarily many individually bounded records.

`slice.go` provides DecodeSlice, which bounds page cardinality before allocation of the next item. Neither
helper retains borrowed input. `reader_test.go` covers lexical boundaries,
streamed chunks, strings and collection limits in hosted CI.
