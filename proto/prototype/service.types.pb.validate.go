// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: prototype/service.types.proto

package prototype

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/anypb"
)

// ensure the imports are used
var (
	_ = bytes.MinRead
	_ = errors.New("")
	_ = fmt.Print
	_ = utf8.UTFMax
	_ = (*regexp.Regexp)(nil)
	_ = (*strings.Reader)(nil)
	_ = net.IPv4len
	_ = time.Duration(0)
	_ = (*url.URL)(nil)
	_ = (*mail.Address)(nil)
	_ = anypb.Any{}
	_ = sort.Sort
)

// Validate checks the field values on BlockValue with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *BlockValue) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on BlockValue with the rules defined in
// the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in BlockValueMultiError, or
// nil if none found.
func (m *BlockValue) ValidateAll() error {
	return m.validate(true)
}

func (m *BlockValue) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Num

	// no validation rules for Hash

	// no validation rules for Parent

	// no validation rules for Timestamp

	// no validation rules for Proposer

	for idx, item := range m.GetTransactions() {
		_, _ = idx, item

		if all {
			switch v := interface{}(item).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, BlockValueValidationError{
						field:  fmt.Sprintf("Transactions[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, BlockValueValidationError{
						field:  fmt.Sprintf("Transactions[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return BlockValueValidationError{
					field:  fmt.Sprintf("Transactions[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	if len(errors) > 0 {
		return BlockValueMultiError(errors)
	}
	return nil
}

// BlockValueMultiError is an error wrapping multiple validation errors
// returned by BlockValue.ValidateAll() if the designated constraints aren't met.
type BlockValueMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m BlockValueMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m BlockValueMultiError) AllErrors() []error { return m }

// BlockValueValidationError is the validation error returned by
// BlockValue.Validate if the designated constraints aren't met.
type BlockValueValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e BlockValueValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e BlockValueValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e BlockValueValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e BlockValueValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e BlockValueValidationError) ErrorName() string { return "BlockValueValidationError" }

// Error satisfies the builtin error interface
func (e BlockValueValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sBlockValue.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = BlockValueValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = BlockValueValidationError{}

// Validate checks the field values on TxValue with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *TxValue) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on TxValue with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in TxValueMultiError, or nil if none found.
func (m *TxValue) ValidateAll() error {
	return m.validate(true)
}

func (m *TxValue) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Num

	// no validation rules for Type

	// no validation rules for Timestamp

	if utf8.RuneCountInString(m.GetHash()) != 66 {
		err := TxValueValidationError{
			field:  "Hash",
			reason: "value length must be 66 runes",
		}
		if !all {
			return err
		}
		errors = append(errors, err)

	}

	if m.GetFee() <= 0 {
		err := TxValueValidationError{
			field:  "Fee",
			reason: "value must be greater than 0",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	// no validation rules for Data

	if len(m.GetInputs()) < 1 {
		err := TxValueValidationError{
			field:  "Inputs",
			reason: "value must contain at least 1 item(s)",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	for idx, item := range m.GetInputs() {
		_, _ = idx, item

		if all {
			switch v := interface{}(item).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, TxValueValidationError{
						field:  fmt.Sprintf("Inputs[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, TxValueValidationError{
						field:  fmt.Sprintf("Inputs[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return TxValueValidationError{
					field:  fmt.Sprintf("Inputs[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	if len(m.GetOutputs()) < 1 {
		err := TxValueValidationError{
			field:  "Outputs",
			reason: "value must contain at least 1 item(s)",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	for idx, item := range m.GetOutputs() {
		_, _ = idx, item

		if all {
			switch v := interface{}(item).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, TxValueValidationError{
						field:  fmt.Sprintf("Outputs[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, TxValueValidationError{
						field:  fmt.Sprintf("Outputs[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return TxValueValidationError{
					field:  fmt.Sprintf("Outputs[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	if len(errors) > 0 {
		return TxValueMultiError(errors)
	}
	return nil
}

// TxValueMultiError is an error wrapping multiple validation errors returned
// by TxValue.ValidateAll() if the designated constraints aren't met.
type TxValueMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m TxValueMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m TxValueMultiError) AllErrors() []error { return m }

// TxValueValidationError is the validation error returned by TxValue.Validate
// if the designated constraints aren't met.
type TxValueValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e TxValueValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e TxValueValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e TxValueValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e TxValueValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e TxValueValidationError) ErrorName() string { return "TxValueValidationError" }

// Error satisfies the builtin error interface
func (e TxValueValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sTxValue.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = TxValueValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = TxValueValidationError{}

// Validate checks the field values on TxInputValue with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *TxInputValue) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on TxInputValue with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in TxInputValueMultiError, or
// nil if none found.
func (m *TxInputValue) ValidateAll() error {
	return m.validate(true)
}

func (m *TxInputValue) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if utf8.RuneCountInString(m.GetHash()) != 66 {
		err := TxInputValueValidationError{
			field:  "Hash",
			reason: "value length must be 66 runes",
		}
		if !all {
			return err
		}
		errors = append(errors, err)

	}

	// no validation rules for Index

	if utf8.RuneCountInString(m.GetAddress()) != 42 {
		err := TxInputValueValidationError{
			field:  "Address",
			reason: "value length must be 42 runes",
		}
		if !all {
			return err
		}
		errors = append(errors, err)

	}

	// no validation rules for Amount

	if len(errors) > 0 {
		return TxInputValueMultiError(errors)
	}
	return nil
}

// TxInputValueMultiError is an error wrapping multiple validation errors
// returned by TxInputValue.ValidateAll() if the designated constraints aren't met.
type TxInputValueMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m TxInputValueMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m TxInputValueMultiError) AllErrors() []error { return m }

// TxInputValueValidationError is the validation error returned by
// TxInputValue.Validate if the designated constraints aren't met.
type TxInputValueValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e TxInputValueValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e TxInputValueValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e TxInputValueValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e TxInputValueValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e TxInputValueValidationError) ErrorName() string { return "TxInputValueValidationError" }

// Error satisfies the builtin error interface
func (e TxInputValueValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sTxInputValue.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = TxInputValueValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = TxInputValueValidationError{}

// Validate checks the field values on TxOutputValue with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *TxOutputValue) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on TxOutputValue with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in TxOutputValueMultiError, or
// nil if none found.
func (m *TxOutputValue) ValidateAll() error {
	return m.validate(true)
}

func (m *TxOutputValue) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if utf8.RuneCountInString(m.GetAddress()) != 42 {
		err := TxOutputValueValidationError{
			field:  "Address",
			reason: "value length must be 42 runes",
		}
		if !all {
			return err
		}
		errors = append(errors, err)

	}

	// no validation rules for Amount

	if utf8.RuneCountInString(m.GetNode()) > 42 {
		err := TxOutputValueValidationError{
			field:  "Node",
			reason: "value length must be at most 42 runes",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return TxOutputValueMultiError(errors)
	}
	return nil
}

// TxOutputValueMultiError is an error wrapping multiple validation errors
// returned by TxOutputValue.ValidateAll() if the designated constraints
// aren't met.
type TxOutputValueMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m TxOutputValueMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m TxOutputValueMultiError) AllErrors() []error { return m }

// TxOutputValueValidationError is the validation error returned by
// TxOutputValue.Validate if the designated constraints aren't met.
type TxOutputValueValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e TxOutputValueValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e TxOutputValueValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e TxOutputValueValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e TxOutputValueValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e TxOutputValueValidationError) ErrorName() string { return "TxOutputValueValidationError" }

// Error satisfies the builtin error interface
func (e TxOutputValueValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sTxOutputValue.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = TxOutputValueValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = TxOutputValueValidationError{}

// Validate checks the field values on SignedTxValue with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *SignedTxValue) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on SignedTxValue with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in SignedTxValueMultiError, or
// nil if none found.
func (m *SignedTxValue) ValidateAll() error {
	return m.validate(true)
}

func (m *SignedTxValue) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if all {
		switch v := interface{}(m.GetData()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, SignedTxValueValidationError{
					field:  "Data",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, SignedTxValueValidationError{
					field:  "Data",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetData()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return SignedTxValueValidationError{
				field:  "Data",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	if utf8.RuneCountInString(m.GetSignature()) != 132 {
		err := SignedTxValueValidationError{
			field:  "Signature",
			reason: "value length must be 132 runes",
		}
		if !all {
			return err
		}
		errors = append(errors, err)

	}

	if len(errors) > 0 {
		return SignedTxValueMultiError(errors)
	}
	return nil
}

// SignedTxValueMultiError is an error wrapping multiple validation errors
// returned by SignedTxValue.ValidateAll() if the designated constraints
// aren't met.
type SignedTxValueMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m SignedTxValueMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m SignedTxValueMultiError) AllErrors() []error { return m }

// SignedTxValueValidationError is the validation error returned by
// SignedTxValue.Validate if the designated constraints aren't met.
type SignedTxValueValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e SignedTxValueValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e SignedTxValueValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e SignedTxValueValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e SignedTxValueValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e SignedTxValueValidationError) ErrorName() string { return "SignedTxValueValidationError" }

// Error satisfies the builtin error interface
func (e SignedTxValueValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sSignedTxValue.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = SignedTxValueValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = SignedTxValueValidationError{}

// Validate checks the field values on UTxO with the rules defined in the proto
// definition for this message. If any rules are violated, the first error
// encountered is returned, or nil if there are no violations.
func (m *UTxO) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on UTxO with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in UTxOMultiError, or nil if none found.
func (m *UTxO) ValidateAll() error {
	return m.validate(true)
}

func (m *UTxO) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for BlockNum

	// no validation rules for Hash

	// no validation rules for Index

	// no validation rules for From

	// no validation rules for To

	// no validation rules for Node

	// no validation rules for Amount

	// no validation rules for Timestamp

	// no validation rules for Txtype

	if len(errors) > 0 {
		return UTxOMultiError(errors)
	}
	return nil
}

// UTxOMultiError is an error wrapping multiple validation errors returned by
// UTxO.ValidateAll() if the designated constraints aren't met.
type UTxOMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m UTxOMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m UTxOMultiError) AllErrors() []error { return m }

// UTxOValidationError is the validation error returned by UTxO.Validate if the
// designated constraints aren't met.
type UTxOValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e UTxOValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e UTxOValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e UTxOValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e UTxOValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e UTxOValidationError) ErrorName() string { return "UTxOValidationError" }

// Error satisfies the builtin error interface
func (e UTxOValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sUTxO.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = UTxOValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = UTxOValidationError{}
