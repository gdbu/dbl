package dbl

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hatchify/stringset"
)

const (
	testDir = "./test_data"
)

var c *Core

func TestNew(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}

	if err = testTeardown(c); err != nil {
		t.Fatal(err)
	}
}

func TestCore_New(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	var entryID string
	if entryID, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	if len(entryID) == 0 {
		t.Fatal("invalid entry id, expected non-empty value")
	}
}

func TestCore_Get(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	var entryID string
	if entryID, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	var fb testStruct
	if err = c.Get(entryID, &fb); err != nil {
		t.Fatal(err)
	}

	if err = testCheck(&foobar, &fb); err != nil {
		t.Fatal(err)
	}
}

func TestCore_Get_context(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	var entryID string
	if entryID, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	type testcase struct {
		iterations int
		timeout    time.Duration
		err        error
	}

	tcs := []testcase{
		{iterations: 1, timeout: time.Millisecond * 200, err: ErrTransactionTimedOut},
		{iterations: 5, timeout: time.Millisecond * 100, err: nil},
		{iterations: 10, timeout: time.Millisecond * 180, err: nil},
		{iterations: 3, timeout: time.Millisecond * 500, err: ErrTransactionTimedOut},
	}

	for _, tc := range tcs {
		ctx := NewTouchContext(context.Background(), time.Millisecond*200)
		if err = c.ReadTransaction(ctx, func(txn *Transaction) (err error) {
			var fb testStruct
			for i := 0; i < tc.iterations; i++ {
				time.Sleep(tc.timeout)

				if err = txn.Get(entryID, &fb); err != nil {
					return
				}
			}

			return
		}); err != tc.err {
			t.Fatalf("invalid error, expected %v and received %v [test case %+v]", tc.err, err, tc)
		}
	}
}

func TestCore_GetByRelationship_users(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	var foobars []*testStruct
	if err = c.GetByRelationship("users", "user_1", &foobars); err != nil {
		t.Fatal(err)
	}

	for _, fb := range foobars {
		if err = testCheck(&foobar, fb); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCore_GetByRelationship_contacts(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	var foobars []*testStruct
	if err = c.GetByRelationship("contacts", "contact_1", &foobars); err != nil {
		t.Fatal(err)
	}

	for _, fb := range foobars {
		if err = testCheck(&foobar, fb); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCore_GetByRelationship_invalid(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	var foobars []*testBadType
	if err = c.GetByRelationship("contacts", "contact_1", &foobars); err != ErrInvalidType {
		t.Fatalf("invalid error, expected %v and received %v", ErrInvalidType, err)
	}
}

func TestCore_GetByRelationship_update(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	var entryID string
	if entryID, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	foobar.UserID = "user_3"

	if err = c.Edit(entryID, &foobar); err != nil {
		t.Fatal(err)
	}

	var foobars []*testStruct
	if err = c.GetByRelationship("users", "user_1", &foobars); err != nil {
		t.Fatal(err)
	}

	if len(foobars) != 0 {
		t.Fatalf("invalid number of entries, expected %d and received %d", 0, len(foobars))
	}

	if err = c.GetByRelationship("users", "user_3", &foobars); err != nil {
		t.Fatal(err)
	}

	if len(foobars) != 1 {
		t.Fatalf("invalid number of entries, expected %d and received %d", 1, len(foobars))
	}

	for _, fb := range foobars {
		if err = testCheck(&foobar, fb); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCore_GetByRelationship_many_to_many(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	entries := []*testStruct{
		newTestStruct("user_1", "contact_1", "group_1", "FOO FOO", "foo", "bar"),
		newTestStruct("user_1", "contact_1", "group_1", "FOO FOO", "bar"),
		newTestStruct("user_1", "contact_1", "group_1", "FOO FOO", "baz"),
	}

	type testcase struct {
		tag           string
		expectedCount int
	}

	runCases := func(cases []testcase) (err error) {
		for _, tc := range cases {
			var entries []*testStruct
			if err = c.GetByRelationship("tags", tc.tag, &entries); err != nil {
				return
			}

			if len(entries) != tc.expectedCount {
				err = fmt.Errorf("invalid number of entries, expected %d and received %d", tc.expectedCount, len(entries))
			}
		}

		return
	}

	createCases := []testcase{
		{
			tag:           "foo",
			expectedCount: 1,
		},
		{
			tag:           "bar",
			expectedCount: 2,
		},
		{
			tag:           "baz",
			expectedCount: 1,
		},
		{
			tag:           "beam",
			expectedCount: 0,
		},
		{
			tag:           "boom",
			expectedCount: 0,
		},
	}

	updateCases := []testcase{
		{
			tag:           "foo",
			expectedCount: 0,
		},
		{
			tag:           "bar",
			expectedCount: 0,
		},
		{
			tag:           "baz",
			expectedCount: 0,
		},
		{
			tag:           "beam",
			expectedCount: 0,
		},
		{
			tag:           "boom",
			expectedCount: 3,
		},
	}

	deleteCases := []testcase{
		{
			tag:           "foo",
			expectedCount: 0,
		},
		{
			tag:           "bar",
			expectedCount: 0,
		},
		{
			tag:           "baz",
			expectedCount: 0,
		},
		{
			tag:           "beam",
			expectedCount: 0,
		},
		{
			tag:           "boom",
			expectedCount: 0,
		},
	}

	for _, entry := range entries {
		if entry.ID, err = c.New(entry); err != nil {
			t.Fatal(err)
		}
	}

	if err = runCases(createCases); err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		entry.Tags = []string{"boom"}
		if err = c.Edit(entry.ID, entry); err != nil {
			t.Fatal(err)
		}
	}

	if err = runCases(updateCases); err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if err = c.Remove(entry.ID); err != nil {
			t.Fatal(err)
		}
	}

	if err = runCases(deleteCases); err != nil {
		t.Fatal(err)
	}
}

func TestCore_GetFirstByRelationship(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	var fb testStruct
	if err = c.GetFirstByRelationship("contacts", foobar.ContactID, &fb); err != nil {
		t.Fatal(err)
	}

	if fb.ID != "00000000" {
		t.Fatalf("invalid ID, expected \"%s\" and recieved \"%s\"", "00000001", fb.ID)
	}

	foobar.ID = fb.ID

	if err = testCheck(&foobar, &fb); err != nil {
		t.Fatal(err)
	}

	return
}

func TestCore_GetLastByRelationship(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	var fb testStruct
	if err = c.GetLastByRelationship("contacts", foobar.ContactID, &fb); err != nil {
		t.Fatal(err)
	}

	if fb.ID != "00000001" {
		t.Fatalf("invalid ID, expected \"%s\" and recieved \"%s\"", "00000001", fb.ID)
	}

	foobar.ID = fb.ID

	if err = testCheck(&foobar, &fb); err != nil {
		t.Fatal(err)
	}

	return
}

func TestCore_Edit(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	var entryID string
	if entryID, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	foobar.Value = "FOO FOO"

	if err = c.Edit(entryID, &foobar); err != nil {
		t.Fatal(err)
	}

	var fb testStruct
	if err = c.Get(entryID, &fb); err != nil {
		t.Fatal(err)
	}

	if err = testCheck(&foobar, &fb); err != nil {
		t.Fatal(err)
	}
}

func TestCore_ForEach(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	var cnt int
	if err = c.ForEach("", func(key string, v Value) (err error) {
		fb := v.(*testStruct)
		// We are not checking ID correctness in this test
		foobar.ID = fb.ID

		if err = testCheck(&foobar, fb); err != nil {
			t.Fatal(err)
		}

		cnt++
		return
	}); err != nil {
		t.Fatal(err)
	}

	if cnt != 2 {
		t.Fatalf("invalid number of entries, expected %d and received %d", 2, cnt)
	}

	return
}

func TestCore_ForEach_with_filter(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	foobar.UserID = "user_2"
	foobar.ContactID = "contact_3"

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	var cnt int
	fn := func(key string, v Value) (err error) {
		fb := v.(*testStruct)
		// We are not checking ID correctness in this test
		foobar.ID = fb.ID

		if err = testCheck(&foobar, fb); err != nil {
			t.Fatal(err)
		}

		cnt++
		return
	}

	if err = c.ForEach("", fn, MakeFilter("contacts", foobar.ContactID, false)); err != nil {
		t.Fatal(err)
	}

	if cnt != 1 {
		t.Fatalf("invalid number of entries, expected %d and received %d", 1, cnt)
	}

	return
}

func TestCore_ForEach_with_multiple_filters(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	user1 := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")
	user2 := makeTestStruct("user_2", "contact_1", "group_1", "bunny bar bar")
	user3 := makeTestStruct("user_3", "contact_2", "group_1", "baz")
	user4 := makeTestStruct("user_4", "contact_2", "group_1", "yep")

	if _, err = c.New(&user1); err != nil {
		t.Fatal(err)
	}

	if _, err = c.New(&user2); err != nil {
		t.Fatal(err)
	}

	if _, err = c.New(&user3); err != nil {
		t.Fatal(err)
	}

	if _, err = c.New(&user4); err != nil {
		t.Fatal(err)
	}

	type testcase struct {
		filters     []Filter
		expectedIDs []string
	}

	tcs := []testcase{
		{
			filters: []Filter{
				MakeFilter("contacts", "contact_1", false),
			},
			expectedIDs: []string{"00000000", "00000001"},
		},
		{
			filters: []Filter{
				MakeFilter("contacts", "contact_2", false),
			},
			expectedIDs: []string{"00000002", "00000003"},
		},
		{
			filters: []Filter{
				MakeFilter("contacts", "contact_1", false),
				MakeFilter("groups", "group_1", false),
			},
			expectedIDs: []string{"00000000", "00000001"},
		},
		{
			filters: []Filter{
				MakeFilter("contacts", "contact_2", false),
				MakeFilter("groups", "group_1", false),
			},
			expectedIDs: []string{"00000002", "00000003"},
		},
		{
			filters: []Filter{
				MakeFilter("contacts", "contact_1", false),
				MakeFilter("users", "user_1", false),
			},
			expectedIDs: []string{"00000000"},
		},
		{
			filters: []Filter{
				MakeFilter("contacts", "contact_2", false),
				MakeFilter("users", "user_2", false),
			},
			expectedIDs: []string{},
		},
		{
			filters: []Filter{
				MakeFilter("contacts", "contact_1", false),
				MakeFilter("users", "user_1", false),
				MakeFilter("groups", "group_1", false),
			},
			expectedIDs: []string{"00000000"},
		},
		{
			filters: []Filter{
				MakeFilter("contacts", "contact_2", false),
				MakeFilter("users", "user_2", false),
				MakeFilter("groups", "group_1", false),
			},
			expectedIDs: []string{},
		},
		{
			filters: []Filter{
				MakeFilter("groups", "group_1", false),
				MakeFilter("contacts", "contact_1", true),
			},
			expectedIDs: []string{"00000002", "00000003"},
		},
		{
			filters: []Filter{
				MakeFilter("groups", "group_1", false),
				MakeFilter("contacts", "contact_2", true),
			},
			expectedIDs: []string{"00000000", "00000001"},
		},
	}

	for _, tc := range tcs {
		ss := stringset.New()
		fn := func(key string, v Value) (err error) {
			ss.Set(key)
			return
		}

		if err = c.ForEach("", fn, tc.filters...); err != nil {
			t.Fatal(err)
		}

		for _, expectedID := range tc.expectedIDs {
			if !ss.Has(expectedID) {
				t.Fatalf("expected ID of %s was not found", expectedID)
			}
		}
	}

	return
}

func TestCore_Cursor(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	var cnt int
	if err = c.Cursor(func(cursor *Cursor) (err error) {
		var fb testStruct
		for err = cursor.Seek("", &fb); err == nil; err = cursor.Next(&fb) {
			// We are not checking ID correctness in this test
			foobar.ID = fb.ID

			if err = testCheck(&foobar, &fb); err != nil {
				break
			}

			cnt++
			fb = testStruct{}
		}

		if err == ErrEndOfEntries {
			err = nil
		}

		return
	}); err != nil {
		t.Fatal(err)
	}

	if cnt != 2 {
		t.Fatalf("invalid number of entries, expected %d and received %d", 2, cnt)
	}

	return
}

func TestCore_Cursor_First(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	if err = c.Cursor(func(cursor *Cursor) (err error) {
		var fb testStruct
		if err = cursor.First(&fb); err != nil {
			return
		}

		if fb.ID != "00000000" {
			return fmt.Errorf("invalid ID, expected \"%s\" and recieved \"%s\"", "00000000", fb.ID)
		}

		foobar.ID = fb.ID

		if err = testCheck(&foobar, &fb); err != nil {
			t.Fatal(err)
		}

		return
	}); err != nil {
		t.Fatal(err)
	}

	return
}

func TestCore_Cursor_Last(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	if err = c.Cursor(func(cursor *Cursor) (err error) {
		var fb testStruct
		if err = cursor.Last(&fb); err != nil {
			return
		}

		if fb.ID != "00000001" {
			return fmt.Errorf("invalid ID, expected \"%s\" and recieved \"%s\"", "00000001", fb.ID)
		}

		foobar.ID = fb.ID

		if err = testCheck(&foobar, &fb); err != nil {
			t.Fatal(err)
		}

		return
	}); err != nil {
		t.Fatal(err)
	}

	return
}

func TestCore_Cursor_Seek(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	if err = c.Cursor(func(cursor *Cursor) (err error) {
		var fb testStruct
		if err = cursor.Seek("00000001", &fb); err != nil {
			return
		}

		if fb.ID != "00000001" {
			return fmt.Errorf("invalid ID, expected \"%s\" and recieved \"%s\"", "00000001", fb.ID)
		}

		foobar.ID = fb.ID

		if err = testCheck(&foobar, &fb); err != nil {
			t.Fatal(err)
		}

		return
	}); err != nil {
		t.Fatal(err)
	}

	return
}

func TestCore_CursorRelationship(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	foobar.UserID = "user_2"
	foobar.ContactID = "contact_3"

	if _, err = c.New(&foobar); err != nil {
		t.Fatal(err)
	}

	var cnt int
	if err = c.CursorRelationship("contacts", foobar.ContactID, func(cursor *Cursor) (err error) {
		var fb testStruct
		for err = cursor.Seek("", &fb); err == nil; err = cursor.Next(&fb) {
			// We are not checking ID correctness in this test
			foobar.ID = fb.ID

			if err = testCheck(&foobar, &fb); err != nil {
				t.Fatal(err)
			}

			cnt++
			fb = testStruct{}
		}

		if err == ErrEndOfEntries {
			err = nil
		}

		return
	}); err != nil {
		t.Fatal(err)
	}

	if cnt != 1 {
		t.Fatalf("invalid number of entries, expected %d and received %d", 1, cnt)
	}

	return
}

func TestCore_Lookups(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	if err = c.SetLookup("test_lookup", "test_0", "foo"); err != nil {
		t.Fatal(err)
	}

	if err = c.SetLookup("test_lookup", "test_0", "bar"); err != nil {
		t.Fatal(err)
	}

	var keys []string
	if keys, err = c.GetLookup("test_lookup", "test_0"); err != nil {
		t.Fatal(err)
	}

	if len(keys) != 2 {
		t.Fatalf("invalid number of keys, expected %d and received %d (%+v)", 2, len(keys), keys)
	}

	for i, key := range keys {
		var expected string
		switch i {
		case 0:
			expected = "bar"
		case 1:
			expected = "foo"
		}

		if expected != key {
			t.Fatalf("invalid key, expected %s and recieved %s", expected, key)
		}
	}

	if err = c.RemoveLookup("test_lookup", "test_0", "foo"); err != nil {
		t.Fatal(err)
	}

	if keys, err = c.GetLookup("test_lookup", "test_0"); err != nil {
		t.Fatal(err)
	}

	if len(keys) != 1 {
		t.Fatalf("invalid number of keys, expected %d and received %d (%+v)", 1, len(keys), keys)
	}

	if keys[0] != "bar" {
		t.Fatalf("invalid key, expected %s and recieved %s", "bar", keys[0])
	}
}

func TestCore_Batch(t *testing.T) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		t.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	var entryID string
	if err = c.Batch(func(txn *Transaction) (err error) {
		entryID, err = txn.New(&foobar)
		return
	}); err != nil {
		t.Fatal(err)
	}

	if err = c.Batch(func(txn *Transaction) (err error) {
		foobar.Value = "foo bar baz"
		err = txn.Edit(entryID, &foobar)
		return
	}); err != nil {
		t.Fatal(err)
	}

	var val testStruct
	if err = c.Get(entryID, &val); err != nil {
		t.Fatal(err)
	}

	if val.Value != "foo bar baz" {
		t.Fatalf("invalid value for Value, expected \"%s\" and received \"%s\"", foobar.Value, val.Value)
	}

	return
}

func BenchmarkCore_New_2(b *testing.B) {
	benchmarkCoreNew(b, 2)
	return
}

func BenchmarkCore_New_4(b *testing.B) {
	benchmarkCoreNew(b, 4)
	return
}

func BenchmarkCore_New_8(b *testing.B) {
	benchmarkCoreNew(b, 8)
	return
}

func BenchmarkCore_New_16(b *testing.B) {
	benchmarkCoreNew(b, 16)
	return
}

func BenchmarkCore_New_32(b *testing.B) {
	benchmarkCoreNew(b, 32)
	return
}

func BenchmarkCore_New_64(b *testing.B) {
	benchmarkCoreNew(b, 64)
	return
}

func BenchmarkCore_Batch_2(b *testing.B) {
	benchmarkCoreBatch(b, 2)
	return
}

func BenchmarkCore_Batch_4(b *testing.B) {
	benchmarkCoreBatch(b, 4)
	return
}

func BenchmarkCore_Batch_8(b *testing.B) {
	benchmarkCoreBatch(b, 8)
	return
}

func BenchmarkCore_Batch_16(b *testing.B) {
	benchmarkCoreBatch(b, 16)
	return
}

func BenchmarkCore_Batch_32(b *testing.B) {
	benchmarkCoreBatch(b, 32)
	return
}

func BenchmarkCore_Batch_64(b *testing.B) {
	benchmarkCoreBatch(b, 64)
	return
}

func benchmarkCoreNew(b *testing.B, threads int) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		b.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	b.SetParallelism(threads)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err = c.New(&foobar); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.ReportAllocs()
	return
}

func benchmarkCoreBatch(b *testing.B, threads int) {
	var (
		c   *Core
		err error
	)

	if c, err = testInit(); err != nil {
		b.Fatal(err)
	}
	defer testTeardown(c)

	foobar := makeTestStruct("user_1", "contact_1", "group_1", "FOO FOO")

	b.SetParallelism(threads)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err = c.Batch(func(txn *Transaction) (err error) {
				_, err = txn.New(&foobar)
				return
			}); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.ReportAllocs()
	return
}

func ExampleNew() {
	var (
		c   *Core
		err error
	)

	if c, err = New("example", "./data", &testStruct{}, "users", "contacts", "groups"); err != nil {
		return
	}

	fmt.Printf("Core! %v\n", c)
}

func ExampleCore_New() {
	var ts testStruct
	ts.Value = "Foo bar"

	var (
		entryID string
		err     error
	)

	if entryID, err = c.New(&ts); err != nil {
		return
	}

	fmt.Printf("New entry! %s\n", entryID)
}

func ExampleCore_Get() {
	var (
		ts  testStruct
		err error
	)

	if err = c.Get("00000000", &ts); err != nil {
		return
	}

	fmt.Printf("Retrieved entry! %+v\n", ts)
}

func ExampleCore_GetByRelationship() {
	var (
		tss []*testStruct
		err error
	)

	if err = c.GetByRelationship("users", "user_1", &tss); err != nil {
		return
	}

	for i, ts := range tss {
		fmt.Printf("Retrieved entry #%d! %+v\n", i, ts)
	}
}

func ExampleCore_ForEach() {
	var err error
	if err = c.ForEach("", func(key string, val Value) (err error) {
		fmt.Printf("Iterating entry (%s)! %+v\n", key, val)
		return
	}); err != nil {
		return
	}
}

func ExampleCore_ForEach_with_filter() {
	var err error
	fn := func(key string, val Value) (err error) {
		fmt.Printf("Iterating entry (%s)! %+v\n", key, val)
		return
	}

	if err = c.ForEach("", fn, MakeFilter("users", "user_1", false)); err != nil {
		return
	}
}

func ExampleCore_Edit() {
	var (
		ts  *testStruct
		err error
	)

	// We will pretend the test struct is already populated

	// Let's update the Value field to "New foo value"
	ts.Value = "New foo value"

	if err = c.Edit("00000000", ts); err != nil {
		return
	}

	fmt.Printf("Edited entry %s!\n", "00000000")
}

func ExampleCore_Remove() {
	var err error
	if err = c.Remove("00000000"); err != nil {
		return
	}

	fmt.Printf("Removed entry %s!\n", "00000000")
}

func testInit() (c *Core, err error) {
	if err = os.MkdirAll(testDir, 0744); err != nil {
		return
	}

	return New("test", testDir, &testStruct{}, "users", "contacts", "groups", "tags")
}

func testTeardown(c *Core) (err error) {
	if err = c.Close(); err != nil {
		return
	}

	return os.RemoveAll(testDir)
}

func testCheck(a, b *testStruct) (err error) {
	if a.ID != b.ID {
		return fmt.Errorf("invalid id, expected %s and received %s", a.ID, b.ID)
	}

	if a.UserID != b.UserID {
		return fmt.Errorf("invalid user id, expected %s and received %s", a.UserID, b.UserID)
	}

	if a.ContactID != b.ContactID {
		return fmt.Errorf("invalid contact id, expected %s and received %s", a.ContactID, b.ContactID)
	}

	if a.Value != b.Value {
		return fmt.Errorf("invalid Value, expected %s and received %s", a.Value, b.Value)
	}

	return
}

func newTestStruct(userID, contactID, groupID, value string, tags ...string) *testStruct {
	t := makeTestStruct(userID, contactID, groupID, value, tags...)
	return &t
}

func makeTestStruct(userID, contactID, groupID, value string, tags ...string) (t testStruct) {
	t.UserID = userID
	t.ContactID = contactID
	t.GroupID = groupID
	t.Value = value
	t.Tags = tags
	return
}

type testStruct struct {
	Entry

	UserID    string   `json:"userID"`
	ContactID string   `json:"contactID"`
	GroupID   string   `json:"groupID"`
	Tags      []string `json:"tags"`

	Value string `json:"value"`
}

func (t *testStruct) GetRelationships() (r Relationships) {
	r.Append(t.UserID)
	r.Append(t.ContactID)
	r.Append(t.GroupID)
	r.Append(t.Tags...)
	return
}

type testBadType struct {
	Foo string
	Bar string
}
