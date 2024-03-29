package output

// Common struct used for rendering JSON summaries of things that just have an ID and a Name
type IdAndName struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

type TableDefinition[T any] struct {
	Header []string
	Row    func(item T) []string
}

// carries conversion functions used by PrintArray and potentially other output code in future
type Mappers[T any] struct {
	// A function which will convert T into an output structure suitable for json.Marshal (e.g. IdAndName).
	// If you leave this as nil, then the command will simply not support output as JSON and will
	// fail if someone asks for it
	Json func(item T) any

	// A function which will convert T into ?? suitable for table printing
	// If you leave this as nil, then the command will simply not support output as
	// a table and will fail if someone asks for it
	Table TableDefinition[T]

	// A function which will convert T into a string suitable for basic text display
	// If you leave this as nil, then the command will simply not support output as basic text and will
	// fail if someone asks for it
	Basic func(item T) string

	// NOTE: We might have some kinds of entities where table formatting doesn't make sense, and we want to
	// render those as basic text instead. This seems unlikely though, defer it until the issue comes up.

	// NOTE: The structure for printing tables would also work for CSV... perhaps we can have --outputFormat=csv for free?
}
