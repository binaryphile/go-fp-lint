package a

// --- positives: value receiver mutates a slice/map field's shared backing,
//     and the type has NO Clone() method ---

// IndexAssignSlice: index-assignment into a slice field corrupts the backing
// array that a value copy shares.
type IndexAssignSlice struct{ items []int }

func (s IndexAssignSlice) Set(i, v int) { s.items[i] = v } // want "Set mutates slice/map field items through a value receiver"

// MapIndexAssign: assigning into a map field mutates the shared map.
type MapIndexAssign struct{ byKey map[string]int }

func (m MapIndexAssign) Put(k string, v int) { m.byKey[k] = v } // want "Put mutates slice/map field byKey through a value receiver"

// MapDelete: delete on a map field mutates the shared map.
type MapDelete struct{ byKey map[string]int }

func (m MapDelete) Remove(k string) { delete(m.byKey, k) } // want "Remove mutates slice/map field byKey through a value receiver"

// ResliceAppend: `f = append(f[:i], ...)` reslices the field — the guide's
// flagship dangerous pattern (corrupts the shared array).
type ResliceAppend struct{ items []int }

func (r ResliceAppend) DropAt(i int) ResliceAppend {
	r.items = append(r.items[:i], r.items[i+1:]...) // want "DropAt mutates slice/map field items through a value receiver"
	return r
}

// --- negatives: not flagged ---

// PointerRecv: pointer receiver — the caller shares the pointer, so mutation
// is intended, not the aliasing trap.
type PointerRecv struct{ items []int }

func (p *PointerRecv) Set(i, v int) { p.items[i] = v }

// HasClone: value receiver mutates a slice field, but the type has a Clone()
// method — the aliasing concern is presumed handled (task heuristic).
type HasClone struct{ items []int }

func (h HasClone) Clone() HasClone {
	h.items = append([]int(nil), h.items...)
	return h
}
func (h HasClone) Set(i, v int) { h.items[i] = v }

// AppendOnly: plain append (no reslice) grows the slice without corrupting a
// shared prefix — guide §11 says Clone() is NOT required.
type AppendOnly struct{ items []int }

func (a AppendOnly) Add(v int) AppendOnly {
	a.items = append(a.items, v)
	return a
}

// ReadOnly: reads the slice field, no mutation.
type ReadOnly struct{ items []int }

func (r ReadOnly) At(i int) int { return r.items[i] }

// ScalarField: value receiver assigns a scalar field — mutates only the local
// copy, no shared backing storage.
type ScalarField struct{ x int }

func (s ScalarField) SetX(x int) { s.x = x }

// LocalSlice: mutates a local slice, not a receiver field.
type LocalSlice struct{ items []int }

func (l LocalSlice) Work() {
	local := []int{1, 2, 3}
	local[0] = 9
	_ = local
}
