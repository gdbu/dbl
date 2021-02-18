package mojura

import (
	"context"
	"fmt"
	"path"
	"reflect"

	"github.com/gdbu/atoms"

	"github.com/hatchify/errors"

	"github.com/mojura/backend"
	"github.com/mojura/kiroku"
)

const (
	// ErrNotInitialized is returned when a service has not been properly initialized
	ErrNotInitialized = errors.Error("service has not been properly initialized")
	// ErrRelationshipNotFound is returned when an relationship is not available for the given relationship key
	ErrRelationshipNotFound = errors.Error("relationship was not found")
	// ErrLookupNotFound is returned when a lookup is not available for the given lookup key
	ErrLookupNotFound = errors.Error("lookup was not found")
	// ErrEntryNotFound is returned when an entry is not available for the given ID
	ErrEntryNotFound = errors.Error("entry was not found")
	// ErrEndOfEntries is returned when a cursor has reached the end of entries
	ErrEndOfEntries = errors.Error("end of entries")
	// ErrInvalidNumberOfRelationships is returned when an invalid number of relationships is provided in a New call
	ErrInvalidNumberOfRelationships = errors.Error("invalid number of relationships")
	// ErrInvalidType is returned when a type which does not match the generator is provided
	ErrInvalidType = errors.Error("invalid type encountered, please check generators")
	// ErrInvalidEntries is returned when a non-slice is presented to GetByRelationship
	ErrInvalidEntries = errors.Error("invalid entries, slice expected")
	// ErrInvalidLogKey is returned when an invalid log key is encountered
	ErrInvalidLogKey = errors.Error("invalid log key, expecting a single :: delimiter")
	// ErrEmptyFilters is returned when relationship pairs are empty for a filter or joined request
	ErrEmptyFilters = errors.Error("invalid relationship pairs, cannot be empty")
	// ErrInversePrimaryFilter is returned when the primary filter is set as an inverse comparison
	ErrInversePrimaryFilter = errors.Error("invalid primary filter, cannot be an inverse comparison")
	// ErrContextCancelled is returned when a transaction ends early from context
	ErrContextCancelled = errors.Error("context cancelled")

	// Break is a non-error which will cause a ForEach loop to break early
	Break = errors.Error("break!")
)

var (
	entriesBktKey       = []byte("entries")
	relationshipsBktKey = []byte("relationships")
	lookupsBktKey       = []byte("lookups")
)

// New will return a new instance of Mojura
func New(name, dir string, example Value, relationships ...string) (cc *Mojura, err error) {
	return NewWithOpts(name, dir, example, defaultOpts, relationships...)
}

// NewWithOpts will return a new instance of Mojura
func NewWithOpts(name, dir string, example Value, opts Opts, relationships ...string) (mp *Mojura, err error) {
	var m Mojura
	if err = opts.Validate(); err != nil {
		return
	}

	if len(example.GetRelationships()) != len(relationships) {
		err = ErrInvalidNumberOfRelationships
		return
	}

	m.opts = &opts
	m.entryType = getMojuraType(example)
	m.indexFmt = fmt.Sprintf("%s0%dd", "%", opts.IndexLength)

	if err = m.init(name, dir, relationships); err != nil {
		return
	}

	if m.k, err = kiroku.New(dir, name, opts.Exporter); err != nil {
		return
	}

	// Initialize new batcher
	m.b = newBatcher(&m)
	// Set return pointer
	mp = &m
	return
}

// Mojura is the DB manager
type Mojura struct {
	db backend.Backend
	k  *kiroku.Kiroku
	b  *batcher

	opts     *Opts
	indexFmt string

	// Element type
	entryType reflect.Type

	relationships [][]byte

	// Closed state
	closed atoms.Bool
}

func (m *Mojura) init(name, dir string, relationships []string) (err error) {
	filename := path.Join(dir, name+".bdb")
	if m.db, err = m.opts.Initializer.New(filename); err != nil {
		return fmt.Errorf("error opening db for %s (%s): %v", name, dir, err)
	}

	err = m.db.Transaction(func(txn backend.Transaction) (err error) {
		if _, err = txn.GetOrCreateBucket(entriesBktKey); err != nil {
			return
		}

		if _, err = txn.GetOrCreateBucket(lookupsBktKey); err != nil {
			return
		}

		var relationshipsBkt backend.Bucket
		if relationshipsBkt, err = txn.GetOrCreateBucket(relationshipsBktKey); err != nil {
			return
		}

		for _, relationship := range relationships {
			rbs := []byte(relationship)
			if _, err = relationshipsBkt.GetOrCreateBucket(rbs); err != nil {
				return
			}

			m.relationships = append(m.relationships, rbs)
		}

		return
	})

	return
}

func (m *Mojura) newReflectValue() (value reflect.Value) {
	// Zero value of the entry type
	return reflect.New(m.entryType)
}

func (m *Mojura) getLastIDAsIndex(entriesBkt backend.Bucket) (lastIndex uint64, err error) {
	// Grab ID from the last Entry
	// Note: We ignore the value because we do not need it
	lastID, _ := entriesBkt.Cursor().Last()

	return parseIDAsIndex(lastID)
}

func (m *Mojura) newEntryValue() (value Value) {
	// Zero value of the entry type
	zero := m.newReflectValue()
	// Interface of zero value
	iface := zero.Interface()
	// Assert as a value (we've confirmed this type at initialization)
	return iface.(Value)
}

func (m *Mojura) marshal(val interface{}) (bs []byte, err error) {
	return m.opts.Encoder.Marshal(val)
}

func (m *Mojura) unmarshal(bs []byte, val interface{}) (err error) {
	return m.opts.Encoder.Unmarshal(bs, val)
}

func (m *Mojura) newValueFromBytes(bs []byte) (val Value, err error) {
	val = m.newEntryValue()
	if err = m.unmarshal(bs, val); err != nil {
		val = nil
		return
	}

	return
}

func (m *Mojura) transaction(fn func(backend.Transaction, *kiroku.Writer) error) (err error) {
	err = m.db.Transaction(func(txn backend.Transaction) (err error) {
		err = m.k.Transaction(func(w *kiroku.Writer) (err error) {
			return fn(txn, w)
		})

		return
	})

	return
}

func (m *Mojura) runTransaction(ctx context.Context, txn backend.Transaction, kw *kiroku.Writer, fn TransactionFn) (err error) {
	t := newTransaction(ctx, m, txn, kw)
	defer t.teardown()
	errCh := make(chan error)

	// Call function from within goroutine
	go func() {
		// Pass returning error to error channel
		errCh <- fn(&t)
	}()

	select {
	case err = <-errCh:
	case <-t.cc.Done():
		// Context is done, attempt to set error from Context
		if err = t.cc.Err(); err != nil {
			return
		}

		err = ErrContextCancelled
	}

	return
}

// New will insert a new entry with the given value and the associated relationships
func (m *Mojura) New(val Value) (entryID string, err error) {
	err = m.Transaction(context.Background(), func(txn *Transaction) (err error) {
		var id []byte
		if id, err = txn.new(val); err != nil {
			return
		}

		entryID = string(id)
		return
	})

	return
}

// Exists will notiy if an entry exists for a given entry ID
func (m *Mojura) Exists(entryID string) (exists bool, err error) {
	err = m.ReadTransaction(context.Background(), func(txn *Transaction) (err error) {
		exists, err = txn.exists([]byte(entryID))
		return
	})

	return
}

// Get will attempt to get an entry by ID
func (m *Mojura) Get(entryID string, val Value) (err error) {
	err = m.ReadTransaction(context.Background(), func(txn *Transaction) (err error) {
		return txn.get([]byte(entryID), val)
	})

	return
}

// GetFiltered will attempt to get the filtered entries
func (m *Mojura) GetFiltered(entries interface{}, o *FilteringOpts) (lastID string, err error) {
	err = m.ReadTransaction(context.Background(), func(txn *Transaction) (err error) {
		lastID, err = txn.GetFiltered(entries, o)
		return
	})

	return
}

// GetFirst will attempt to get the first entry which matches the provided filters
// Note: Will return ErrEntryNotFound if no match is found
func (m *Mojura) GetFirst(val Value, o *IteratingOpts) (err error) {
	if err = m.ReadTransaction(context.Background(), func(txn *Transaction) (err error) {
		return txn.getFirst(val, o)
	}); err != nil {
		return
	}

	return
}

// GetLast will attempt to get the last entry which matches the provided filters
// Note: Will return ErrEntryNotFound if no match is found
func (m *Mojura) GetLast(val Value, o *IteratingOpts) (err error) {
	if err = m.ReadTransaction(context.Background(), func(txn *Transaction) (err error) {
		return txn.getLast(val, o)
	}); err != nil {
		return
	}

	return
}

// ForEach will iterate through each of the entries
func (m *Mojura) ForEach(fn ForEachFn, o *IteratingOpts) (err error) {
	err = m.ReadTransaction(context.Background(), func(txn *Transaction) (err error) {
		return txn.ForEach(fn, o)
	})

	return
}

// ForEachID will iterate through each of the entry IDs
func (m *Mojura) ForEachID(fn ForEachIDFn, o *IteratingOpts) (err error) {
	err = m.ReadTransaction(context.Background(), func(txn *Transaction) (err error) {
		return txn.ForEachID(fn, o)
	})

	return
}

// Cursor will return an iterating cursor
func (m *Mojura) Cursor(fn func(Cursor) error, fs ...Filter) (err error) {
	if err = m.ReadTransaction(context.Background(), func(txn *Transaction) (err error) {
		var c Cursor
		if c, err = txn.cursor(fs); err != nil {
			return
		}

		return fn(c)
	}); err == Break {
		err = nil
	}

	return
}

// Edit will attempt to edit an entry by ID
func (m *Mojura) Edit(entryID string, val Value) (err error) {
	err = m.Transaction(context.Background(), func(txn *Transaction) (err error) {
		return txn.edit([]byte(entryID), val)
	})

	return
}

// Remove will remove a relationship ID and it's related relationship IDs
func (m *Mojura) Remove(entryID string) (err error) {
	err = m.Transaction(context.Background(), func(txn *Transaction) (err error) {
		return txn.remove([]byte(entryID))
	})

	return
}

// Transaction will initialize a transaction
func (m *Mojura) Transaction(ctx context.Context, fn func(*Transaction) error) (err error) {
	err = m.transaction(func(txn backend.Transaction, kw *kiroku.Writer) (err error) {
		return m.runTransaction(ctx, txn, kw, fn)
	})

	return
}

// ReadTransaction will initialize a read-only transaction
func (m *Mojura) ReadTransaction(ctx context.Context, fn func(*Transaction) error) (err error) {
	err = m.db.ReadTransaction(func(txn backend.Transaction) (err error) {
		return m.runTransaction(ctx, txn, nil, fn)
	})

	return
}

// Batch will initialize a batch
func (m *Mojura) Batch(ctx context.Context, fn func(*Transaction) error) (err error) {
	return <-m.b.Append(ctx, fn)
}

// Close will close the selected instance of Mojura
func (m *Mojura) Close() (err error) {
	if !m.closed.Set(true) {
		return errors.ErrIsClosed
	}

	var errs errors.ErrorList
	errs.Push(m.db.Close())
	errs.Push(m.k.Close())
	return errs.Err()
}
