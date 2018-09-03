package accounting

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/asyncreader"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interfaces
var (
	_ io.ReadCloser = &Account{}
	_ io.Reader     = &accountStream{}
	_ Accounter     = &Account{}
	_ Accounter     = &accountStream{}
)

func TestNewAccountSizeName(t *testing.T) {
	in := ioutil.NopCloser(bytes.NewBuffer([]byte{1}))
	acc := NewAccountSizeName(in, 1, "test")
	assert.Equal(t, in, acc.in)
	assert.Equal(t, acc, Stats.inProgress.get("test"))
	err := acc.Close()
	assert.NoError(t, err)
	assert.Nil(t, Stats.inProgress.get("test"))
}

func TestNewAccount(t *testing.T) {
	obj := mockobject.Object("test")
	in := ioutil.NopCloser(bytes.NewBuffer([]byte{1}))
	acc := NewAccount(in, obj)
	assert.Equal(t, in, acc.in)
	assert.Equal(t, acc, Stats.inProgress.get("test"))
	err := acc.Close()
	assert.NoError(t, err)
	assert.Nil(t, Stats.inProgress.get("test"))
}

func TestAccountWithBuffer(t *testing.T) {
	in := ioutil.NopCloser(bytes.NewBuffer([]byte{1}))

	acc := NewAccountSizeName(in, -1, "test")
	acc.WithBuffer()
	// should have a buffer for an unknown size
	_, ok := acc.in.(*asyncreader.AsyncReader)
	require.True(t, ok)
	assert.NoError(t, acc.Close())

	acc = NewAccountSizeName(in, 1, "test")
	acc.WithBuffer()
	// should not have a buffer for a small size
	_, ok = acc.in.(*asyncreader.AsyncReader)
	require.False(t, ok)
	assert.NoError(t, acc.Close())
}

func TestAccountGetUpdateReader(t *testing.T) {
	in := ioutil.NopCloser(bytes.NewBuffer([]byte{1}))
	acc := NewAccountSizeName(in, 1, "test")

	assert.Equal(t, in, acc.GetReader())

	in2 := ioutil.NopCloser(bytes.NewBuffer([]byte{1}))
	acc.UpdateReader(in2)

	assert.Equal(t, in2, acc.GetReader())

	assert.NoError(t, acc.Close())
}

func TestAccountRead(t *testing.T) {
	in := ioutil.NopCloser(bytes.NewBuffer([]byte{1, 2, 3}))
	acc := NewAccountSizeName(in, 1, "test")

	assert.True(t, acc.start.IsZero())
	assert.Equal(t, 0, acc.lpBytes)
	assert.Equal(t, int64(0), acc.bytes)
	assert.Equal(t, int64(0), Stats.bytes)

	var buf = make([]byte, 2)
	n, err := acc.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte{1, 2}, buf[:n])

	assert.False(t, acc.start.IsZero())
	assert.Equal(t, 2, acc.lpBytes)
	assert.Equal(t, int64(2), acc.bytes)
	assert.Equal(t, int64(2), Stats.bytes)

	n, err = acc.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, []byte{3}, buf[:n])

	n, err = acc.Read(buf)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)

	assert.NoError(t, acc.Close())
}

func TestAccountString(t *testing.T) {
	in := ioutil.NopCloser(bytes.NewBuffer([]byte{1, 2, 3}))
	acc := NewAccountSizeName(in, 3, "test")

	// FIXME not an exhaustive test!

	assert.Equal(t, "test:  0% /3, 0/s, -", strings.TrimSpace(acc.String()))

	var buf = make([]byte, 2)
	n, err := acc.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)

	assert.Equal(t, "test: 66% /3, 0/s, -", strings.TrimSpace(acc.String()))

	assert.NoError(t, acc.Close())
}

// Test the Accounter interface methods on Account and accountStream
func TestAccountAccounter(t *testing.T) {
	in := ioutil.NopCloser(bytes.NewBuffer([]byte{1, 2, 3}))
	acc := NewAccountSizeName(in, 3, "test")

	assert.True(t, in == acc.OldStream())

	in2 := ioutil.NopCloser(bytes.NewBuffer([]byte{2, 3, 4}))

	acc.SetStream(in2)
	assert.True(t, in2 == acc.OldStream())

	r := acc.WrapStream(in)
	as, ok := r.(Accounter)
	require.True(t, ok)
	assert.True(t, in == as.OldStream())
	assert.True(t, in2 == acc.OldStream())
	accs, ok := r.(*accountStream)
	require.True(t, ok)
	assert.Equal(t, acc, accs.acc)
	assert.True(t, in == accs.in)

	// Check Read on the accountStream
	var buf = make([]byte, 2)
	n, err := r.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte{1, 2}, buf[:n])

	// Test that we can get another accountstream out
	in3 := ioutil.NopCloser(bytes.NewBuffer([]byte{3, 1, 2}))
	r2 := as.WrapStream(in3)
	as2, ok := r2.(Accounter)
	require.True(t, ok)
	assert.True(t, in3 == as2.OldStream())
	assert.True(t, in2 == acc.OldStream())
	accs2, ok := r2.(*accountStream)
	require.True(t, ok)
	assert.Equal(t, acc, accs2.acc)
	assert.True(t, in3 == accs2.in)

	// Test we can set this new accountStream
	as2.SetStream(in)
	assert.True(t, in == as2.OldStream())

	// Test UnWrap on accountStream
	unwrapped, wrap := UnWrap(r2)
	assert.True(t, unwrapped == in)
	r3 := wrap(in2)
	assert.True(t, in2 == r3.(Accounter).OldStream())

	// TestUnWrap on a normal io.Reader
	unwrapped, wrap = UnWrap(in2)
	assert.True(t, unwrapped == in2)
	assert.True(t, wrap(in3) == in3)

}

func TestAccountMaxTransfer(t *testing.T) {
	old := fs.Config.MaxTransfer
	fs.Config.MaxTransfer = 15
	defer func() {
		fs.Config.MaxTransfer = old
	}()
	Stats.ResetCounters()

	in := ioutil.NopCloser(bytes.NewBuffer(make([]byte, 100)))
	acc := NewAccountSizeName(in, 1, "test")

	var b = make([]byte, 10)

	n, err := acc.Read(b)
	assert.Equal(t, 10, n)
	assert.NoError(t, err)
	n, err = acc.Read(b)
	assert.Equal(t, 10, n)
	assert.NoError(t, err)
	n, err = acc.Read(b)
	assert.Equal(t, 0, n)
	assert.Equal(t, ErrorMaxTransferLimitReached, err)
	assert.True(t, fserrors.IsFatalError(err))
}
