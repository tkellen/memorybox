package file

// List provides a listing of files that can be reasoned about with memorybox
// semantics. It also satisfies the sort.Sortable interface.
type List []*File

// Len returns the length of the underlying array.
func (l List) Len() int { return len(l) }

// Less returns which of two indexes in the array is "smaller" alphanumerically.
func (l List) Less(i, j int) bool { return l[i].Name < l[j].Name }

// Swap re-orders the underlying array (used by sort.Sort).
func (l List) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

// Filter returns a new index where each file has been kept or discarded on
// based on the result of processing it with the supplied predicate function.
func (l List) Filter(fn func(*File) bool) List {
	var result List
	for _, file := range l {
		if fn(file) {
			result = append(result, file)
			continue
		}
	}
	return result
}

// Meta produces a new file list that only contains metafiles.
func (l List) Meta() List {
	return l.Filter(func(file *File) bool {
		return IsMetaFileName(file.Name)
	})
}

// Data produces a new file list that only contains datafiles.
func (l List) Data() List {
	return l.Filter(func(file *File) bool {
		return !IsMetaFileName(file.Name)
	})
}

// ByName produces a map keyed by filename of all files in the list.
func (l List) ByName() map[string]*File {
	result := make(map[string]*File, len(l))
	for _, file := range l {
		name := file.Name
		result[name] = file
	}
	return result
}

// Names produces an array of filenames in the same order as the file list.
func (l List) Names() []string {
	result := make([]string, len(l))
	for index, file := range l {
		result[index] = file.Name
	}
	return result
}

// Invalid returns a list of files that lack a metafile or datafile pair.
func (l List) Invalid() List {
	return l.paired(false)
}

// Valid returns a list of files have a metafile or datafile pair.
func (l List) Valid() List {
	return l.paired(true)
}

func (l List) paired(hasPair bool) List {
	index := l.ByName()
	return l.Filter(func(file *File) bool {
		pair := MetaNameFrom(file.Name)
		if IsMetaFileName(file.Name) {
			pair = DataNameFrom(file.Name)
		}
		_, ok := index[pair]
		return ok == hasPair
	})
}
