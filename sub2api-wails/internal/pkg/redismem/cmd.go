package redismem

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

var Nil = errors.New("redis: nil")

type Z struct {
	Score  float64
	Member interface{}
}

type StatusCmd struct {
	err error
}

func newStatusCmd(err error) *StatusCmd {
	return &StatusCmd{err: err}
}

func (c *StatusCmd) Err() error        { return c.err }
func (c *StatusCmd) Result() (string, error) { return "OK", c.err }
func (c *StatusCmd) Val() string       { return "OK" }
func (c *StatusCmd) String() string    { return "OK" }

type IntCmd struct {
	val int64
	err error
}

func newIntCmd(val int64, err error) *IntCmd {
	return &IntCmd{val: val, err: err}
}

func (c *IntCmd) Err() error                 { return c.err }
func (c *IntCmd) Result() (int64, error)     { return c.val, c.err }
func (c *IntCmd) Val() int64                 { return c.val }
func (c *IntCmd) Int() (int, error)          { return int(c.val), c.err }
func (c *IntCmd) Int64() (int64, error)      { return c.val, c.err }
func (c *IntCmd) Uint64() (uint64, error)    { return uint64(c.val), c.err }
func (c *IntCmd) String() string             { return strconv.FormatInt(c.val, 10) }

type StringCmd struct {
	val string
	err error
}

func newStringCmd(val string, err error) *StringCmd {
	return &StringCmd{val: val, err: err}
}

func (c *StringCmd) Err() error              { return c.err }
func (c *StringCmd) Result() (string, error) { return c.val, c.err }
func (c *StringCmd) Val() string             { return c.val }
func (c *StringCmd) Int() (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	n, err := strconv.Atoi(c.val)
	if err != nil {
		return 0, err
	}
	return n, nil
}
func (c *StringCmd) Int64() (int64, error) {
	if c.err != nil {
		return 0, c.err
	}
	n, err := strconv.ParseInt(c.val, 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}
func (c *StringCmd) Uint64() (uint64, error) {
	if c.err != nil {
		return 0, c.err
	}
	n, err := strconv.ParseUint(c.val, 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}
func (c *StringCmd) Float64() (float64, error) {
	if c.err != nil {
		return 0, c.err
	}
	n, err := strconv.ParseFloat(c.val, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}
func (c *StringCmd) Bytes() ([]byte, error) {
	if c.err != nil {
		return nil, c.err
	}
	return []byte(c.val), nil
}
func (c *StringCmd) Bool() (bool, error) {
	if c.err != nil {
		return false, c.err
	}
	return c.val == "1" || c.val == "true", nil
}
func (c *StringCmd) String() string { return c.val }

type BoolCmd struct {
	val bool
	err error
}

func newBoolCmd(val bool, err error) *BoolCmd {
	return &BoolCmd{val: val, err: err}
}

func (c *BoolCmd) Err() error             { return c.err }
func (c *BoolCmd) Result() (bool, error)  { return c.val, c.err }
func (c *BoolCmd) Val() bool              { return c.val }
func (c *BoolCmd) String() string         { return fmt.Sprintf("%v", c.val) }

type DurationCmd struct {
	val time.Duration
	err error
}

func newDurationCmd(val time.Duration, err error) *DurationCmd {
	return &DurationCmd{val: val, err: err}
}

func (c *DurationCmd) Err() error                    { return c.err }
func (c *DurationCmd) Result() (time.Duration, error) { return c.val, c.err }
func (c *DurationCmd) Val() time.Duration            { return c.val }
func (c *DurationCmd) String() string                { return c.val.String() }

type TimeCmd struct {
	val time.Time
	err error
}

func newTimeCmd(val time.Time, err error) *TimeCmd {
	return &TimeCmd{val: val, err: err}
}

func (c *TimeCmd) Err() error               { return c.err }
func (c *TimeCmd) Result() (time.Time, error) { return c.val, c.err }
func (c *TimeCmd) Val() time.Time           { return c.val }

type StringSliceCmd struct {
	val []string
	err error
}

func newStringSliceCmd(val []string, err error) *StringSliceCmd {
	return &StringSliceCmd{val: val, err: err}
}

func (c *StringSliceCmd) Err() error               { return c.err }
func (c *StringSliceCmd) Result() ([]string, error) { return c.val, c.err }
func (c *StringSliceCmd) Val() []string             { return c.val }

type StringStringMapCmd struct {
	val map[string]string
	err error
}

func newStringStringMapCmd(val map[string]string, err error) *StringStringMapCmd {
	return &StringStringMapCmd{val: val, err: err}
}

func (c *StringStringMapCmd) Err() error                            { return c.err }
func (c *StringStringMapCmd) Result() (map[string]string, error)    { return c.val, c.err }
func (c *StringStringMapCmd) Val() map[string]string                { return c.val }

type SliceCmd struct {
	val []interface{}
	err error
}

func newSliceCmd(val []interface{}, err error) *SliceCmd {
	return &SliceCmd{val: val, err: err}
}

func (c *SliceCmd) Err() error                    { return c.err }
func (c *SliceCmd) Result() ([]interface{}, error) { return c.val, c.err }
func (c *SliceCmd) Val() []interface{}            { return c.val }

type ScanCmd struct {
	keys   []string
	cursor uint64
	err    error
}

func newScanCmd(keys []string, cursor uint64, err error) *ScanCmd {
	return &ScanCmd{keys: keys, cursor: cursor, err: err}
}

func (c *ScanCmd) Err() error                          { return c.err }
func (c *ScanCmd) Result() ([]string, uint64, error)   { return c.keys, c.cursor, c.err }

type Cmd struct {
	val interface{}
	err error
}

func newCmd(val interface{}, err error) *Cmd {
	return &Cmd{val: val, err: err}
}

func (c *Cmd) Err() error                      { return c.err }
func (c *Cmd) Result() (interface{}, error)     { return c.val, c.err }
func (c *Cmd) Val() interface{}                { return c.val }
func (c *Cmd) Int() (int, error) {
	if c.err != nil {
		return 0, c.err
	}
	switch v := c.val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		n, err := strconv.Atoi(v)
		return n, err
	default:
		return 0, fmt.Errorf("redis: unexpected type %T for Int()", c.val)
	}
}
func (c *Cmd) Int64() (int64, error) {
	if c.err != nil {
		return 0, c.err
	}
	switch v := c.val.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		return n, err
	default:
		return 0, fmt.Errorf("redis: unexpected type %T for Int64()", c.val)
	}
}
func (c *Cmd) Uint64() (uint64, error) {
	if c.err != nil {
		return 0, c.err
	}
	switch v := c.val.(type) {
	case uint64:
		return v, nil
	case int64:
		return uint64(v), nil
	case int:
		return uint64(v), nil
	case float64:
		return uint64(v), nil
	case string:
		n, err := strconv.ParseUint(v, 10, 64)
		return n, err
	default:
		return 0, fmt.Errorf("redis: unexpected type %T for Uint64()", c.val)
	}
}
func (c *Cmd) Float64() (float64, error) {
	if c.err != nil {
		return 0, c.err
	}
	switch v := c.val.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		n, err := strconv.ParseFloat(v, 64)
		return n, err
	default:
		return 0, fmt.Errorf("redis: unexpected type %T for Float64()", c.val)
	}
}
func (c *Cmd) String() string {
	if c.err != nil {
		return ""
	}
	return fmt.Sprintf("%v", c.val)
}
func (c *Cmd) Bool() (bool, error) {
	if c.err != nil {
		return false, c.err
	}
	switch v := c.val.(type) {
	case bool:
		return v, nil
	case int:
		return v != 0, nil
	case int64:
		return v != 0, nil
	default:
		return false, fmt.Errorf("redis: unexpected type %T for Bool()", c.val)
	}
}
func (c *Cmd) Bytes() ([]byte, error) {
	if c.err != nil {
		return nil, c.err
	}
	switch v := c.val.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		return nil, fmt.Errorf("redis: unexpected type %T for Bytes()", c.val)
	}
}
