// Integration tests - test rclone by doing real transactions to a
// storage provider to and from the local disk.
//
// By default it will use a local fs, however you can provide a
// -remote option to use a different remote.  The test_all.go script
// is a wrapper to call this for all the test remotes.
//
// FIXME not safe for concurrent running of tests until fs.Config is
// no longer a global
//
// NB When writing tests
//
// Make sure every series of writes to the remote has a
// fstest.CheckItems() before use.  This make sure the directory
// listing is now consistent and stops cascading errors.
//
// Call accounting.Stats.ResetCounters() before every fs.Sync() as it
// uses the error count internally.

package operations_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strings"
	"testing"
	"time"

	_ "github.com/ncw/rclone/backend/all" // import all backends
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/filter"
	"github.com/ncw/rclone/fs/hash"
	"github.com/ncw/rclone/fs/list"
	"github.com/ncw/rclone/fs/operations"
	"github.com/ncw/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Some times used in the tests
var (
	t1 = fstest.Time("2001-02-03T04:05:06.499999999Z")
	t2 = fstest.Time("2011-12-25T12:59:59.123456789Z")
	t3 = fstest.Time("2011-12-30T12:59:59.000000000Z")
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

func TestMkdir(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	err := operations.Mkdir(r.Fremote, "")
	require.NoError(t, err)
	fstest.CheckListing(t, r.Fremote, []fstest.Item{})

	err = operations.Mkdir(r.Fremote, "")
	require.NoError(t, err)
}

func TestLsd(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("sub dir/hello world", "hello world", t1)

	fstest.CheckItems(t, r.Fremote, file1)

	var buf bytes.Buffer
	err := operations.ListDir(r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	assert.Contains(t, res, "sub dir\n")
}

func TestLs(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	var buf bytes.Buffer
	err := operations.List(r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	assert.Contains(t, res, "        0 empty space\n")
	assert.Contains(t, res, "       60 potato2\n")
}

func TestLsLong(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	var buf bytes.Buffer
	err := operations.ListLong(r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	lines := strings.Split(strings.Trim(res, "\n"), "\n")
	assert.Equal(t, 2, len(lines))

	timeFormat := "2006-01-02 15:04:05.000000000"
	precision := r.Fremote.Precision()
	location := time.Now().Location()
	checkTime := func(m, filename string, expected time.Time) {
		modTime, err := time.ParseInLocation(timeFormat, m, location) // parse as localtime
		if err != nil {
			t.Errorf("Error parsing %q: %v", m, err)
		} else {
			dt, ok := fstest.CheckTimeEqualWithPrecision(expected, modTime, precision)
			if !ok {
				t.Errorf("%s: Modification time difference too big |%s| > %s (%s vs %s) (precision %s)", filename, dt, precision, modTime, expected, precision)
			}
		}
	}

	m1 := regexp.MustCompile(`(?m)^        0 (\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{9}) empty space$`)
	if ms := m1.FindStringSubmatch(res); ms == nil {
		t.Errorf("empty space missing: %q", res)
	} else {
		checkTime(ms[1], "empty space", t2.Local())
	}

	m2 := regexp.MustCompile(`(?m)^       60 (\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{9}) potato2$`)
	if ms := m2.FindStringSubmatch(res); ms == nil {
		t.Errorf("potato2 missing: %q", res)
	} else {
		checkTime(ms[1], "potato2", t1.Local())
	}
}

func TestHashSums(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	// MD5 Sum

	var buf bytes.Buffer
	err := operations.Md5sum(r.Fremote, &buf)
	require.NoError(t, err)
	res := buf.String()
	if !strings.Contains(res, "d41d8cd98f00b204e9800998ecf8427e  empty space\n") &&
		!strings.Contains(res, "                     UNSUPPORTED  empty space\n") &&
		!strings.Contains(res, "                                  empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "d6548b156ea68a4e003e786df99eee76  potato2\n") &&
		!strings.Contains(res, "                     UNSUPPORTED  potato2\n") &&
		!strings.Contains(res, "                                  potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}

	// SHA1 Sum

	buf.Reset()
	err = operations.Sha1sum(r.Fremote, &buf)
	require.NoError(t, err)
	res = buf.String()
	if !strings.Contains(res, "da39a3ee5e6b4b0d3255bfef95601890afd80709  empty space\n") &&
		!strings.Contains(res, "                             UNSUPPORTED  empty space\n") &&
		!strings.Contains(res, "                                          empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "9dc7f7d3279715991a22853f5981df582b7f9f6d  potato2\n") &&
		!strings.Contains(res, "                             UNSUPPORTED  potato2\n") &&
		!strings.Contains(res, "                                          potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}

	// Dropbox Hash Sum

	buf.Reset()
	err = operations.DropboxHashSum(r.Fremote, &buf)
	require.NoError(t, err)
	res = buf.String()
	if !strings.Contains(res, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  empty space\n") &&
		!strings.Contains(res, "                                                     UNSUPPORTED  empty space\n") &&
		!strings.Contains(res, "                                                                  empty space\n") {
		t.Errorf("empty space missing: %q", res)
	}
	if !strings.Contains(res, "a979481df794fed9c3990a6a422e0b1044ac802c15fab13af9c687f8bdbee01a  potato2\n") &&
		!strings.Contains(res, "                                                     UNSUPPORTED  potato2\n") &&
		!strings.Contains(res, "                                                                  potato2\n") {
		t.Errorf("potato2 missing: %q", res)
	}
}

func TestCount(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("potato2", "------------------------------------------------------------", t1)
	file2 := r.WriteBoth("empty space", "", t2)
	file3 := r.WriteBoth("sub dir/potato3", "hello", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2, file3)

	// Check the MaxDepth too
	fs.Config.MaxDepth = 1
	defer func() { fs.Config.MaxDepth = -1 }()

	objects, size, err := operations.Count(r.Fremote)
	require.NoError(t, err)
	assert.Equal(t, int64(2), objects)
	assert.Equal(t, int64(60), size)
}

func TestDelete(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("small", "1234567890", t2)                                                                                           // 10 bytes
	file2 := r.WriteObject("medium", "------------------------------------------------------------", t1)                                        // 60 bytes
	file3 := r.WriteObject("large", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", t1) // 100 bytes
	fstest.CheckItems(t, r.Fremote, file1, file2, file3)

	filter.Active.Opt.MaxSize = 60
	defer func() {
		filter.Active.Opt.MaxSize = -1
	}()

	err := operations.Delete(r.Fremote)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Fremote, file3)
}

func testCheck(t *testing.T, checkFunction func(fdst, fsrc fs.Fs, oneway bool) error) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	check := func(i int, wantErrors int64, oneway bool) {
		fs.Debugf(r.Fremote, "%d: Starting check test", i)
		oldErrors := accounting.Stats.GetErrors()
		err := checkFunction(r.Fremote, r.Flocal, oneway)
		gotErrors := accounting.Stats.GetErrors() - oldErrors
		if wantErrors == 0 && err != nil {
			t.Errorf("%d: Got error when not expecting one: %v", i, err)
		}
		if wantErrors != 0 && err == nil {
			t.Errorf("%d: No error when expecting one", i)
		}
		if wantErrors != gotErrors {
			t.Errorf("%d: Expecting %d errors but got %d", i, wantErrors, gotErrors)
		}
		fs.Debugf(r.Fremote, "%d: Ending check test", i)
	}

	file1 := r.WriteBoth("rutabaga", "is tasty", t3)
	fstest.CheckItems(t, r.Fremote, file1)
	fstest.CheckItems(t, r.Flocal, file1)
	check(1, 0, false)

	file2 := r.WriteFile("potato2", "------------------------------------------------------------", t1)
	fstest.CheckItems(t, r.Flocal, file1, file2)
	check(2, 1, false)

	file3 := r.WriteObject("empty space", "", t2)
	fstest.CheckItems(t, r.Fremote, file1, file3)
	check(3, 2, false)

	file2r := file2
	if fs.Config.SizeOnly {
		file2r = r.WriteObject("potato2", "--Some-Differences-But-Size-Only-Is-Enabled-----------------", t1)
	} else {
		r.WriteObject("potato2", "------------------------------------------------------------", t1)
	}
	fstest.CheckItems(t, r.Fremote, file1, file2r, file3)
	check(4, 1, false)

	r.WriteFile("empty space", "", t2)
	fstest.CheckItems(t, r.Flocal, file1, file2, file3)
	check(5, 0, false)

	file4 := r.WriteObject("remotepotato", "------------------------------------------------------------", t1)
	fstest.CheckItems(t, r.Fremote, file1, file2r, file3, file4)
	check(6, 1, false)
	check(7, 0, true)
}

func TestCheck(t *testing.T) {
	testCheck(t, operations.Check)
}

func TestCheckDownload(t *testing.T) {
	testCheck(t, operations.CheckDownload)
}

func TestCheckSizeOnly(t *testing.T) {
	fs.Config.SizeOnly = true
	defer func() { fs.Config.SizeOnly = false }()
	TestCheck(t)
}

func TestCat(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteBoth("file1", "ABCDEFGHIJ", t1)
	file2 := r.WriteBoth("file2", "012345678", t2)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	for _, test := range []struct {
		offset int64
		count  int64
		a      string
		b      string
	}{
		{0, -1, "ABCDEFGHIJ", "012345678"},
		{0, 5, "ABCDE", "01234"},
		{-3, -1, "HIJ", "678"},
		{1, 3, "BCD", "123"},
	} {
		var buf bytes.Buffer
		err := operations.Cat(r.Fremote, &buf, test.offset, test.count)
		require.NoError(t, err)
		res := buf.String()

		if res != test.a+test.b && res != test.b+test.a {
			t.Errorf("Incorrect output from Cat(%d,%d): %q", test.offset, test.count, res)
		}
	}
}

func TestRcat(t *testing.T) {
	checkSumBefore := fs.Config.CheckSum
	defer func() { fs.Config.CheckSum = checkSumBefore }()

	check := func(withChecksum bool) {
		fs.Config.CheckSum = withChecksum
		prefix := "no_checksum_"
		if withChecksum {
			prefix = "with_checksum_"
		}

		r := fstest.NewRun(t)
		defer r.Finalise()

		fstest.CheckListing(t, r.Fremote, []fstest.Item{})

		data1 := "this is some really nice test data"
		path1 := prefix + "small_file_from_pipe"

		data2 := string(make([]byte, fs.Config.StreamingUploadCutoff+1))
		path2 := prefix + "big_file_from_pipe"

		in := ioutil.NopCloser(strings.NewReader(data1))
		_, err := operations.Rcat(r.Fremote, path1, in, t1)
		require.NoError(t, err)

		in = ioutil.NopCloser(strings.NewReader(data2))
		_, err = operations.Rcat(r.Fremote, path2, in, t2)
		require.NoError(t, err)

		file1 := fstest.NewItem(path1, data1, t1)
		file2 := fstest.NewItem(path2, data2, t2)
		fstest.CheckItems(t, r.Fremote, file1, file2)
	}

	check(true)
	check(false)
}

func TestRmdirsNoLeaveRoot(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	r.Mkdir(r.Fremote)

	// Make some files and dirs we expect to keep
	r.ForceMkdir(r.Fremote)
	file1 := r.WriteObject("A1/B1/C1/one", "aaa", t1)
	//..and dirs we expect to delete
	require.NoError(t, operations.Mkdir(r.Fremote, "A2"))
	require.NoError(t, operations.Mkdir(r.Fremote, "A1/B2"))
	require.NoError(t, operations.Mkdir(r.Fremote, "A1/B2/C2"))
	require.NoError(t, operations.Mkdir(r.Fremote, "A1/B1/C3"))
	require.NoError(t, operations.Mkdir(r.Fremote, "A3"))
	require.NoError(t, operations.Mkdir(r.Fremote, "A3/B3"))
	require.NoError(t, operations.Mkdir(r.Fremote, "A3/B3/C4"))
	//..and one more file at the end
	file2 := r.WriteObject("A1/two", "bbb", t2)

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{
			file1, file2,
		},
		[]string{
			"A1",
			"A1/B1",
			"A1/B1/C1",
			"A2",
			"A1/B2",
			"A1/B2/C2",
			"A1/B1/C3",
			"A3",
			"A3/B3",
			"A3/B3/C4",
		},
		fs.GetModifyWindow(r.Fremote),
	)

	require.NoError(t, operations.Rmdirs(r.Fremote, "", false))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{
			file1, file2,
		},
		[]string{
			"A1",
			"A1/B1",
			"A1/B1/C1",
		},
		fs.GetModifyWindow(r.Fremote),
	)

}

func TestRmdirsLeaveRoot(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	r.Mkdir(r.Fremote)

	r.ForceMkdir(r.Fremote)

	require.NoError(t, operations.Mkdir(r.Fremote, "A1"))
	require.NoError(t, operations.Mkdir(r.Fremote, "A1/B1"))
	require.NoError(t, operations.Mkdir(r.Fremote, "A1/B1/C1"))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{
			"A1",
			"A1/B1",
			"A1/B1/C1",
		},
		fs.GetModifyWindow(r.Fremote),
	)

	require.NoError(t, operations.Rmdirs(r.Fremote, "A1", true))

	fstest.CheckListingWithPrecision(
		t,
		r.Fremote,
		[]fstest.Item{},
		[]string{
			"A1",
		},
		fs.GetModifyWindow(r.Fremote),
	)
}

func TestRcatSize(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	const body = "------------------------------------------------------------"
	file1 := r.WriteFile("potato1", body, t1)
	file2 := r.WriteFile("potato2", body, t2)
	// Test with known length
	bodyReader := ioutil.NopCloser(strings.NewReader(body))
	obj, err := operations.RcatSize(r.Fremote, file1.Path, bodyReader, int64(len(body)), file1.ModTime)
	require.NoError(t, err)
	assert.Equal(t, int64(len(body)), obj.Size())
	assert.Equal(t, file1.Path, obj.Remote())

	// Test with unknown length
	bodyReader = ioutil.NopCloser(strings.NewReader(body)) // reset Reader
	ioutil.NopCloser(strings.NewReader(body))
	obj, err = operations.RcatSize(r.Fremote, file2.Path, bodyReader, -1, file2.ModTime)
	require.NoError(t, err)
	assert.Equal(t, int64(len(body)), obj.Size())
	assert.Equal(t, file2.Path, obj.Remote())

	// Check files exist
	fstest.CheckItems(t, r.Fremote, file1, file2)
}

func TestMoveFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	file1 := r.WriteFile("file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	file2 := file1
	file2.Path = "sub/file2"

	err := operations.MoveFile(r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file2)

	r.WriteFile("file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	err = operations.MoveFile(r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file2)

	err = operations.MoveFile(r.Fremote, r.Fremote, file2.Path, file2.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal)
	fstest.CheckItems(t, r.Fremote, file2)
}

func TestCopyFile(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	file1 := r.WriteFile("file1", "file1 contents", t1)
	fstest.CheckItems(t, r.Flocal, file1)

	file2 := file1
	file2.Path = "sub/file2"

	err := operations.CopyFile(r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)

	err = operations.CopyFile(r.Fremote, r.Flocal, file2.Path, file1.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)

	err = operations.CopyFile(r.Fremote, r.Fremote, file2.Path, file2.Path)
	require.NoError(t, err)
	fstest.CheckItems(t, r.Flocal, file1)
	fstest.CheckItems(t, r.Fremote, file2)
}

// testFsInfo is for unit testing fs.Info
type testFsInfo struct {
	name      string
	root      string
	stringVal string
	precision time.Duration
	hashes    hash.Set
	features  fs.Features
}

// Name of the remote (as passed into NewFs)
func (i *testFsInfo) Name() string { return i.name }

// Root of the remote (as passed into NewFs)
func (i *testFsInfo) Root() string { return i.root }

// String returns a description of the FS
func (i *testFsInfo) String() string { return i.stringVal }

// Precision of the ModTimes in this Fs
func (i *testFsInfo) Precision() time.Duration { return i.precision }

// Returns the supported hash types of the filesystem
func (i *testFsInfo) Hashes() hash.Set { return i.hashes }

// Returns the supported hash types of the filesystem
func (i *testFsInfo) Features() *fs.Features { return &i.features }

func TestSameConfig(t *testing.T) {
	a := &testFsInfo{name: "name", root: "root"}
	for _, test := range []struct {
		name     string
		root     string
		expected bool
	}{
		{"name", "root", true},
		{"name", "rooty", true},
		{"namey", "root", false},
		{"namey", "roott", false},
	} {
		b := &testFsInfo{name: test.name, root: test.root}
		actual := operations.SameConfig(a, b)
		assert.Equal(t, test.expected, actual)
		actual = operations.SameConfig(b, a)
		assert.Equal(t, test.expected, actual)
	}
}

func TestSame(t *testing.T) {
	a := &testFsInfo{name: "name", root: "root"}
	for _, test := range []struct {
		name     string
		root     string
		expected bool
	}{
		{"name", "root", true},
		{"name", "rooty", false},
		{"namey", "root", false},
		{"namey", "roott", false},
	} {
		b := &testFsInfo{name: test.name, root: test.root}
		actual := operations.Same(a, b)
		assert.Equal(t, test.expected, actual)
		actual = operations.Same(b, a)
		assert.Equal(t, test.expected, actual)
	}
}

func TestOverlapping(t *testing.T) {
	a := &testFsInfo{name: "name", root: "root"}
	for _, test := range []struct {
		name     string
		root     string
		expected bool
	}{
		{"name", "root", true},
		{"namey", "root", false},
		{"name", "rooty", false},
		{"namey", "rooty", false},
		{"name", "roo", false},
		{"name", "root/toot", true},
		{"name", "root/toot/", true},
		{"name", "", true},
		{"name", "/", true},
	} {
		b := &testFsInfo{name: test.name, root: test.root}
		what := fmt.Sprintf("(%q,%q) vs (%q,%q)", a.name, a.root, b.name, b.root)
		actual := operations.Overlapping(a, b)
		assert.Equal(t, test.expected, actual, what)
		actual = operations.Overlapping(b, a)
		assert.Equal(t, test.expected, actual, what)
	}
}

type errorReader struct {
	err error
}

func (er errorReader) Read(p []byte) (n int, err error) {
	return 0, er.err
}

func TestCheckEqualReaders(t *testing.T) {
	b65a := make([]byte, 65*1024)
	b65b := make([]byte, 65*1024)
	b65b[len(b65b)-1] = 1
	b66 := make([]byte, 66*1024)

	differ, err := operations.CheckEqualReaders(bytes.NewBuffer(b65a), bytes.NewBuffer(b65a))
	assert.NoError(t, err)
	assert.Equal(t, differ, false)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b65a), bytes.NewBuffer(b65b))
	assert.NoError(t, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b65a), bytes.NewBuffer(b66))
	assert.NoError(t, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b66), bytes.NewBuffer(b65a))
	assert.NoError(t, err)
	assert.Equal(t, differ, true)

	myErr := errors.New("sentinel")
	wrap := func(b []byte) io.Reader {
		r := bytes.NewBuffer(b)
		e := errorReader{myErr}
		return io.MultiReader(r, e)
	}

	differ, err = operations.CheckEqualReaders(wrap(b65a), bytes.NewBuffer(b65a))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(wrap(b65a), bytes.NewBuffer(b65b))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(wrap(b65a), bytes.NewBuffer(b66))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(wrap(b66), bytes.NewBuffer(b65a))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b65a), wrap(b65a))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b65a), wrap(b65b))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b65a), wrap(b66))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)

	differ, err = operations.CheckEqualReaders(bytes.NewBuffer(b66), wrap(b65a))
	assert.Equal(t, myErr, err)
	assert.Equal(t, differ, true)
}

func TestListFormat(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()
	file1 := r.WriteObject("a", "a", t1)
	file2 := r.WriteObject("subdir/b", "b", t1)

	fstest.CheckItems(t, r.Fremote, file1, file2)

	items, _ := list.DirSorted(r.Fremote, true, "")
	var list operations.ListFormat
	list.AddPath()
	list.SetDirSlash(false)
	assert.Equal(t, "subdir", list.Format(items[1]))

	list.SetDirSlash(true)
	assert.Equal(t, "subdir/", list.Format(items[1]))

	list.SetOutput(nil)
	assert.Equal(t, "", list.Format(items[1]))

	list.AppendOutput(func() string { return "a" })
	list.AppendOutput(func() string { return "b" })
	assert.Equal(t, "ab", list.Format(items[1]))
	list.SetSeparator(":::")
	assert.Equal(t, "a:::b", list.Format(items[1]))

	list.SetOutput(nil)
	list.AddModTime()
	assert.Equal(t, items[0].ModTime().Local().Format("2006-01-02 15:04:05"), list.Format(items[0]))

	list.SetOutput(nil)
	list.AddID()
	_ = list.Format(items[0]) // Can't really check anything - at least it didn't panic!

	list.SetOutput(nil)
	list.AddMimeType()
	assert.Contains(t, list.Format(items[0]), "/")
	assert.Equal(t, "inode/directory", list.Format(items[1]))

	list.SetOutput(nil)
	list.AddPath()
	list.SetAbsolute(true)
	assert.Equal(t, "/a", list.Format(items[0]))
	list.SetAbsolute(false)
	assert.Equal(t, "a", list.Format(items[0]))

	list.SetOutput(nil)
	list.AddSize()
	assert.Equal(t, "1", list.Format(items[0]))

	list.AddPath()
	list.AddModTime()
	list.SetDirSlash(true)
	list.SetSeparator("__SEP__")
	assert.Equal(t, "1__SEP__a__SEP__"+items[0].ModTime().Local().Format("2006-01-02 15:04:05"), list.Format(items[0]))
	assert.Equal(t, fmt.Sprintf("%d", items[1].Size())+"__SEP__subdir/__SEP__"+items[1].ModTime().Local().Format("2006-01-02 15:04:05"), list.Format(items[1]))

	for _, test := range []struct {
		ht   hash.Type
		want string
	}{
		{hash.MD5, "0cc175b9c0f1b6a831c399e269772661"},
		{hash.SHA1, "86f7e437faa5a7fce15d1ddcb9eaeaea377667b8"},
		{hash.Dropbox, "bf5d3affb73efd2ec6c36ad3112dd933efed63c4e1cbffcfa88e2759c144f2d8"},
	} {
		list.SetOutput(nil)
		list.AddHash(test.ht)
		got := list.Format(items[0])
		if got != "UNSUPPORTED" && got != "" {
			assert.Equal(t, test.want, got)
		}
	}

	list.SetOutput(nil)
	list.SetSeparator("|")
	list.SetCSV(true)
	list.AddSize()
	list.AddPath()
	list.AddModTime()
	list.SetDirSlash(true)
	assert.Equal(t, "1|a|"+items[0].ModTime().Local().Format("2006-01-02 15:04:05"), list.Format(items[0]))
	assert.Equal(t, fmt.Sprintf("%d", items[1].Size())+"|subdir/|"+items[1].ModTime().Local().Format("2006-01-02 15:04:05"), list.Format(items[1]))

}
