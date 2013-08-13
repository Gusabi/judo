package jsoncodec_test

import (
	"encoding/json"
	"errors"
	"io"
	. "launchpad.net/gocheck"
	"launchpad.net/juju-core/rpc"
	"launchpad.net/juju-core/rpc/jsoncodec"
	"launchpad.net/juju-core/testing"
	"reflect"
	"regexp"
	stdtesting "testing"
)

type suite struct {
	testing.LoggingSuite
}

var _ = Suite(&suite{})

func TestPackage(t *stdtesting.T) {
	TestingT(t)
}

type value struct {
	X string
}

var readTests = []struct {
	msg        string
	expectHdr  rpc.Header
	expectBody interface{}
}{{
	msg: `{"RequestId": 1, "Type": "foo", "Id": "id", "Request": "frob", "Params": {"X": "param"}}`,
	expectHdr: rpc.Header{
		RequestId: 1,
		Type:      "foo",
		Id:        "id",
		Request:   "frob",
	},
	expectBody: &value{X: "param"},
}, {
	msg: `{"RequestId": 2, "Error": "an error", "ErrorCode": "a code"}`,
	expectHdr: rpc.Header{
		RequestId: 2,
		Error:     "an error",
		ErrorCode: "a code",
	},
	expectBody: new(map[string]interface{}),
}, {
	msg: `{"RequestId": 3, "Response": {"X": "result"}}`,
	expectHdr: rpc.Header{
		RequestId: 3,
	},
	expectBody: &value{X: "result"},
}}

func (*suite) TestRead(c *C) {
	for i, test := range readTests {
		c.Logf("test %d", i)
		codec := jsoncodec.New(&testConn{
			readMsgs: []string{test.msg},
		})
		var hdr rpc.Header
		err := codec.ReadHeader(&hdr)
		c.Assert(err, IsNil)
		c.Assert(hdr, DeepEquals, test.expectHdr)

		c.Assert(hdr.IsRequest(), Equals, test.expectHdr.IsRequest())

		body := reflect.New(reflect.ValueOf(test.expectBody).Type().Elem()).Interface()
		err = codec.ReadBody(body, test.expectHdr.IsRequest())
		c.Assert(err, IsNil)
		c.Assert(body, DeepEquals, test.expectBody)

		err = codec.ReadHeader(&hdr)
		c.Assert(err, Equals, io.EOF)
	}
}

func (*suite) TestReadHeaderLogsRequests(c *C) {
	msg := `{"RequestId":1,"Type": "foo","Id": "id","Request":"frob","Params":{"X":"param"}}`
	codec := jsoncodec.New(&testConn{
		readMsgs: []string{msg, msg, msg},
	})
	// Check that logging is off by default
	var h rpc.Header
	err := codec.ReadHeader(&h)
	c.Assert(err, IsNil)
	c.Assert(c.GetTestLog(), Matches, "")

	// Check that we see a log message when we switch logging on.
	codec.SetLogging(true)
	err = codec.ReadHeader(&h)
	c.Assert(err, IsNil)
	c.Assert(c.GetTestLog(), Matches, ".*DEBUG juju rpc/jsoncodec: <- "+regexp.QuoteMeta(msg)+`\n`)

	// Check that we can switch it off again
	codec.SetLogging(false)
	err = codec.ReadHeader(&h)
	c.Assert(err, IsNil)
	c.Assert(c.GetTestLog(), Matches, ".*DEBUG juju rpc/jsoncodec: <- "+regexp.QuoteMeta(msg)+`\n`)
}

func (*suite) TestWriteMessageLogsRequests(c *C) {
	codec := jsoncodec.New(&testConn{})
	h := rpc.Header{
		RequestId: 1,
		Type:      "foo",
		Id:        "id",
		Request:   "frob",
	}

	// Check that logging is off by default
	err := codec.WriteMessage(&h, value{X: "param"})
	c.Assert(err, IsNil)
	c.Assert(c.GetTestLog(), Matches, "")

	// Check that we see a log message when we switch logging on.
	codec.SetLogging(true)
	err = codec.WriteMessage(&h, value{X: "param"})
	c.Assert(err, IsNil)
	msg := `{"RequestId":1,"Type":"foo","Id":"id","Request":"frob","Params":{"X":"param"}}`
	c.Assert(c.GetTestLog(), Matches, `.*DEBUG juju rpc/jsoncodec: -> `+regexp.QuoteMeta(msg)+`\n`)

	// Check that we can switch it off again
	codec.SetLogging(false)
	err = codec.WriteMessage(&h, value{X: "param"})
	c.Assert(err, IsNil)
	c.Assert(c.GetTestLog(), Matches, `.*DEBUG juju rpc/jsoncodec: -> `+regexp.QuoteMeta(msg)+`\n`)
}

func (*suite) TestConcurrentSetLoggingAndWrite(c *C) {
	// If log messages are not set atomically, this
	// test will fail when run under the race detector.
	codec := jsoncodec.New(&testConn{})
	done := make(chan struct{})
	go func() {
		codec.SetLogging(true)
		done <- struct{}{}
	}()
	h := rpc.Header{
		RequestId: 1,
		Type:      "foo",
		Id:        "id",
		Request:   "frob",
	}
	err := codec.WriteMessage(&h, value{X: "param"})
	c.Assert(err, IsNil)
	<-done
}

func (*suite) TestConcurrentSetLoggingAndRead(c *C) {
	// If log messages are not set atomically, this
	// test will fail when run under the race detector.
	msg := `{"RequestId":1,"Type": "foo","Id": "id","Request":"frob","Params":{"X":"param"}}`
	codec := jsoncodec.New(&testConn{
		readMsgs: []string{msg, msg, msg},
	})
	done := make(chan struct{})
	go func() {
		codec.SetLogging(true)
		done <- struct{}{}
	}()
	var h rpc.Header
	err := codec.ReadHeader(&h)
	c.Assert(err, IsNil)
	<-done
}

func (*suite) TestErrorAfterClose(c *C) {
	conn := &testConn{
		err: errors.New("some error"),
	}
	codec := jsoncodec.New(conn)
	var hdr rpc.Header
	err := codec.ReadHeader(&hdr)
	c.Assert(err, ErrorMatches, "error receiving message: some error")

	err = codec.Close()
	c.Assert(err, IsNil)
	c.Assert(conn.closed, Equals, true)

	err = codec.ReadHeader(&hdr)
	c.Assert(err, Equals, io.EOF)
}

var writeTests = []struct {
	hdr       *rpc.Header
	body      interface{}
	isRequest bool
	expect    string
}{{
	hdr: &rpc.Header{
		RequestId: 1,
		Type:      "foo",
		Id:        "id",
		Request:   "frob",
	},
	body:   &value{X: "param"},
	expect: `{"RequestId": 1, "Type": "foo","Id":"id", "Request": "frob", "Params": {"X": "param"}}`,
}, {
	hdr: &rpc.Header{
		RequestId: 2,
		Error:     "an error",
		ErrorCode: "a code",
	},
	expect: `{"RequestId": 2, "Error": "an error", "ErrorCode": "a code"}`,
}, {
	hdr: &rpc.Header{
		RequestId: 3,
	},
	body:   &value{X: "result"},
	expect: `{"RequestId": 3, "Response": {"X": "result"}}`,
}}

func (*suite) TestWrite(c *C) {
	for i, test := range writeTests {
		c.Logf("test %d", i)
		var conn testConn
		codec := jsoncodec.New(&conn)
		err := codec.WriteMessage(test.hdr, test.body)
		c.Assert(err, IsNil)
		c.Assert(conn.writeMsgs, HasLen, 1)

		assertJSONEqual(c, conn.writeMsgs[0], test.expect)
	}
}

// assertJSONEqual compares the json strings v0
// and v1 ignoring white space.
func assertJSONEqual(c *C, v0, v1 string) {
	var m0, m1 interface{}
	err := json.Unmarshal([]byte(v0), &m0)
	c.Assert(err, IsNil)
	err = json.Unmarshal([]byte(v1), &m1)
	c.Assert(err, IsNil)
	data0, err := json.Marshal(m0)
	c.Assert(err, IsNil)
	data1, err := json.Marshal(m1)
	c.Assert(err, IsNil)
	c.Assert(string(data0), Equals, string(data1))
}

type testConn struct {
	readMsgs  []string
	err       error
	writeMsgs []string
	closed    bool
}

func (c *testConn) Receive(msg interface{}) error {
	if len(c.readMsgs) > 0 {
		s := c.readMsgs[0]
		c.readMsgs = c.readMsgs[1:]
		return json.Unmarshal([]byte(s), msg)
	}
	if c.err != nil {
		return c.err
	}
	return io.EOF
}

func (c *testConn) Send(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.writeMsgs = append(c.writeMsgs, string(data))
	return nil
}

func (c *testConn) Close() error {
	c.closed = true
	return nil
}
