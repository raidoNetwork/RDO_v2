// Code generated by protoc-gen-validate. DO NOT EDIT.
// source: prototype/types.proto

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

// Validate checks the field values on Block with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *Block) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Block with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in BlockMultiError, or nil if none found.
func (m *Block) ValidateAll() error {
	return m.validate(true)
}

func (m *Block) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Num

	// no validation rules for Slot

	// no validation rules for Version

	// no validation rules for Hash

	// no validation rules for Parent

	// no validation rules for Timestamp

	// no validation rules for Txroot

	if all {
		switch v := interface{}(m.GetProposer()).(type) {
		case interface{ ValidateAll() error }:
			if err := v.ValidateAll(); err != nil {
				errors = append(errors, BlockValidationError{
					field:  "Proposer",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		case interface{ Validate() error }:
			if err := v.Validate(); err != nil {
				errors = append(errors, BlockValidationError{
					field:  "Proposer",
					reason: "embedded message failed validation",
					cause:  err,
				})
			}
		}
	} else if v, ok := interface{}(m.GetProposer()).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return BlockValidationError{
				field:  "Proposer",
				reason: "embedded message failed validation",
				cause:  err,
			}
		}
	}

	for idx, item := range m.GetApprovers() {
		_, _ = idx, item

		if all {
			switch v := interface{}(item).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, BlockValidationError{
						field:  fmt.Sprintf("Approvers[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, BlockValidationError{
						field:  fmt.Sprintf("Approvers[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return BlockValidationError{
					field:  fmt.Sprintf("Approvers[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	for idx, item := range m.GetSlashers() {
		_, _ = idx, item

		if all {
			switch v := interface{}(item).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, BlockValidationError{
						field:  fmt.Sprintf("Slashers[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, BlockValidationError{
						field:  fmt.Sprintf("Slashers[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return BlockValidationError{
					field:  fmt.Sprintf("Slashers[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	for idx, item := range m.GetTransactions() {
		_, _ = idx, item

		if all {
			switch v := interface{}(item).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, BlockValidationError{
						field:  fmt.Sprintf("Transactions[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, BlockValidationError{
						field:  fmt.Sprintf("Transactions[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return BlockValidationError{
					field:  fmt.Sprintf("Transactions[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	if len(errors) > 0 {
		return BlockMultiError(errors)
	}
	return nil
}

// BlockMultiError is an error wrapping multiple validation errors returned by
// Block.ValidateAll() if the designated constraints aren't met.
type BlockMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m BlockMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m BlockMultiError) AllErrors() []error { return m }

// BlockValidationError is the validation error returned by Block.Validate if
// the designated constraints aren't met.
type BlockValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e BlockValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e BlockValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e BlockValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e BlockValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e BlockValidationError) ErrorName() string { return "BlockValidationError" }

// Error satisfies the builtin error interface
func (e BlockValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sBlock.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = BlockValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = BlockValidationError{}

// Validate checks the field values on Sign with the rules defined in the proto
// definition for this message. If any rules are violated, the first error
// encountered is returned, or nil if there are no violations.
func (m *Sign) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Sign with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in SignMultiError, or nil if none found.
func (m *Sign) ValidateAll() error {
	return m.validate(true)
}

func (m *Sign) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for Address

	// no validation rules for Signature

	if len(errors) > 0 {
		return SignMultiError(errors)
	}
	return nil
}

// SignMultiError is an error wrapping multiple validation errors returned by
// Sign.ValidateAll() if the designated constraints aren't met.
type SignMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m SignMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m SignMultiError) AllErrors() []error { return m }

// SignValidationError is the validation error returned by Sign.Validate if the
// designated constraints aren't met.
type SignValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e SignValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e SignValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e SignValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e SignValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e SignValidationError) ErrorName() string { return "SignValidationError" }

// Error satisfies the builtin error interface
func (e SignValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sSign.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = SignValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = SignValidationError{}

// Validate checks the field values on Transaction with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *Transaction) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Transaction with the rules defined in
// the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in TransactionMultiError, or
// nil if none found.
func (m *Transaction) ValidateAll() error {
	return m.validate(true)
}

func (m *Transaction) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if m.GetNum() < 0 {
		err := TransactionValidationError{
			field:  "Num",
			reason: "value must be greater than or equal to 0",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if _, ok := _Transaction_Type_InLookup[m.GetType()]; !ok {
		err := TransactionValidationError{
			field:  "Type",
			reason: "value must be in list [1 5 6]",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if m.GetTimestamp() <= 0 {
		err := TransactionValidationError{
			field:  "Timestamp",
			reason: "value must be greater than 0",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if len(m.GetHash()) != 32 {
		err := TransactionValidationError{
			field:  "Hash",
			reason: "value length must be 32 bytes",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if m.GetFee() <= 0 {
		err := TransactionValidationError{
			field:  "Fee",
			reason: "value must be greater than 0",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if len(m.GetData()) > 10000 {
		err := TransactionValidationError{
			field:  "Data",
			reason: "value length must be at most 10000 bytes",
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
					errors = append(errors, TransactionValidationError{
						field:  fmt.Sprintf("Inputs[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, TransactionValidationError{
						field:  fmt.Sprintf("Inputs[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return TransactionValidationError{
					field:  fmt.Sprintf("Inputs[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	for idx, item := range m.GetOutputs() {
		_, _ = idx, item

		if all {
			switch v := interface{}(item).(type) {
			case interface{ ValidateAll() error }:
				if err := v.ValidateAll(); err != nil {
					errors = append(errors, TransactionValidationError{
						field:  fmt.Sprintf("Outputs[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			case interface{ Validate() error }:
				if err := v.Validate(); err != nil {
					errors = append(errors, TransactionValidationError{
						field:  fmt.Sprintf("Outputs[%v]", idx),
						reason: "embedded message failed validation",
						cause:  err,
					})
				}
			}
		} else if v, ok := interface{}(item).(interface{ Validate() error }); ok {
			if err := v.Validate(); err != nil {
				return TransactionValidationError{
					field:  fmt.Sprintf("Outputs[%v]", idx),
					reason: "embedded message failed validation",
					cause:  err,
				}
			}
		}

	}

	if len(m.GetSignature()) != 65 {
		err := TransactionValidationError{
			field:  "Signature",
			reason: "value length must be 65 bytes",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	// no validation rules for Status

	if len(errors) > 0 {
		return TransactionMultiError(errors)
	}
	return nil
}

// TransactionMultiError is an error wrapping multiple validation errors
// returned by Transaction.ValidateAll() if the designated constraints aren't met.
type TransactionMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m TransactionMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m TransactionMultiError) AllErrors() []error { return m }

// TransactionValidationError is the validation error returned by
// Transaction.Validate if the designated constraints aren't met.
type TransactionValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e TransactionValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e TransactionValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e TransactionValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e TransactionValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e TransactionValidationError) ErrorName() string { return "TransactionValidationError" }

// Error satisfies the builtin error interface
func (e TransactionValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sTransaction.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = TransactionValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = TransactionValidationError{}

var _Transaction_Type_InLookup = map[uint32]struct{}{
	1: {},
	5: {},
	6: {},
}

// Validate checks the field values on TxInput with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *TxInput) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on TxInput with the rules defined in the
// proto definition for this message. If any rules are violated, the result is
// a list of violation errors wrapped in TxInputMultiError, or nil if none found.
func (m *TxInput) ValidateAll() error {
	return m.validate(true)
}

func (m *TxInput) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(m.GetHash()) != 32 {
		err := TxInputValidationError{
			field:  "Hash",
			reason: "value length must be 32 bytes",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	// no validation rules for Index

	if len(m.GetAddress()) != 20 {
		err := TxInputValidationError{
			field:  "Address",
			reason: "value length must be 20 bytes",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if m.GetAmount() <= 0 {
		err := TxInputValidationError{
			field:  "Amount",
			reason: "value must be greater than 0",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return TxInputMultiError(errors)
	}
	return nil
}

// TxInputMultiError is an error wrapping multiple validation errors returned
// by TxInput.ValidateAll() if the designated constraints aren't met.
type TxInputMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m TxInputMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m TxInputMultiError) AllErrors() []error { return m }

// TxInputValidationError is the validation error returned by TxInput.Validate
// if the designated constraints aren't met.
type TxInputValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e TxInputValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e TxInputValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e TxInputValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e TxInputValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e TxInputValidationError) ErrorName() string { return "TxInputValidationError" }

// Error satisfies the builtin error interface
func (e TxInputValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sTxInput.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = TxInputValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = TxInputValidationError{}

// Validate checks the field values on TxOutput with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *TxOutput) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on TxOutput with the rules defined in
// the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in TxOutputMultiError, or nil
// if none found.
func (m *TxOutput) ValidateAll() error {
	return m.validate(true)
}

func (m *TxOutput) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	if len(m.GetAddress()) != 20 {
		err := TxOutputValidationError{
			field:  "Address",
			reason: "value length must be 20 bytes",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if m.GetAmount() <= 0 {
		err := TxOutputValidationError{
			field:  "Amount",
			reason: "value must be greater than 0",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if len(m.GetNode()) > 0 {

		if len(m.GetNode()) != 20 {
			err := TxOutputValidationError{
				field:  "Node",
				reason: "value length must be 20 bytes",
			}
			if !all {
				return err
			}
			errors = append(errors, err)
		}

	}

	if len(errors) > 0 {
		return TxOutputMultiError(errors)
	}
	return nil
}

// TxOutputMultiError is an error wrapping multiple validation errors returned
// by TxOutput.ValidateAll() if the designated constraints aren't met.
type TxOutputMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m TxOutputMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m TxOutputMultiError) AllErrors() []error { return m }

// TxOutputValidationError is the validation error returned by
// TxOutput.Validate if the designated constraints aren't met.
type TxOutputValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e TxOutputValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e TxOutputValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e TxOutputValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e TxOutputValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e TxOutputValidationError) ErrorName() string { return "TxOutputValidationError" }

// Error satisfies the builtin error interface
func (e TxOutputValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sTxOutput.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = TxOutputValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = TxOutputValidationError{}

// Validate checks the field values on Metadata with the rules defined in the
// proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *Metadata) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on Metadata with the rules defined in
// the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in MetadataMultiError, or nil
// if none found.
func (m *Metadata) ValidateAll() error {
	return m.validate(true)
}

func (m *Metadata) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for HeadSlot

	// no validation rules for HeadBlockNum

	if len(m.GetHeadBlockHash()) != 32 {
		err := MetadataValidationError{
			field:  "HeadBlockHash",
			reason: "value length must be 32 bytes",
		}
		if !all {
			return err
		}
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return MetadataMultiError(errors)
	}
	return nil
}

// MetadataMultiError is an error wrapping multiple validation errors returned
// by Metadata.ValidateAll() if the designated constraints aren't met.
type MetadataMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m MetadataMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m MetadataMultiError) AllErrors() []error { return m }

// MetadataValidationError is the validation error returned by
// Metadata.Validate if the designated constraints aren't met.
type MetadataValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e MetadataValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e MetadataValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e MetadataValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e MetadataValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e MetadataValidationError) ErrorName() string { return "MetadataValidationError" }

// Error satisfies the builtin error interface
func (e MetadataValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sMetadata.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = MetadataValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = MetadataValidationError{}

// Validate checks the field values on BlockRequest with the rules defined in
// the proto definition for this message. If any rules are violated, the first
// error encountered is returned, or nil if there are no violations.
func (m *BlockRequest) Validate() error {
	return m.validate(false)
}

// ValidateAll checks the field values on BlockRequest with the rules defined
// in the proto definition for this message. If any rules are violated, the
// result is a list of violation errors wrapped in BlockRequestMultiError, or
// nil if none found.
func (m *BlockRequest) ValidateAll() error {
	return m.validate(true)
}

func (m *BlockRequest) validate(all bool) error {
	if m == nil {
		return nil
	}

	var errors []error

	// no validation rules for StartSlot

	// no validation rules for Count

	// no validation rules for Step

	if len(errors) > 0 {
		return BlockRequestMultiError(errors)
	}
	return nil
}

// BlockRequestMultiError is an error wrapping multiple validation errors
// returned by BlockRequest.ValidateAll() if the designated constraints aren't met.
type BlockRequestMultiError []error

// Error returns a concatenation of all the error messages it wraps.
func (m BlockRequestMultiError) Error() string {
	var msgs []string
	for _, err := range m {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// AllErrors returns a list of validation violation errors.
func (m BlockRequestMultiError) AllErrors() []error { return m }

// BlockRequestValidationError is the validation error returned by
// BlockRequest.Validate if the designated constraints aren't met.
type BlockRequestValidationError struct {
	field  string
	reason string
	cause  error
	key    bool
}

// Field function returns field value.
func (e BlockRequestValidationError) Field() string { return e.field }

// Reason function returns reason value.
func (e BlockRequestValidationError) Reason() string { return e.reason }

// Cause function returns cause value.
func (e BlockRequestValidationError) Cause() error { return e.cause }

// Key function returns key value.
func (e BlockRequestValidationError) Key() bool { return e.key }

// ErrorName returns error name.
func (e BlockRequestValidationError) ErrorName() string { return "BlockRequestValidationError" }

// Error satisfies the builtin error interface
func (e BlockRequestValidationError) Error() string {
	cause := ""
	if e.cause != nil {
		cause = fmt.Sprintf(" | caused by: %v", e.cause)
	}

	key := ""
	if e.key {
		key = "key for "
	}

	return fmt.Sprintf(
		"invalid %sBlockRequest.%s: %s%s",
		key,
		e.field,
		e.reason,
		cause)
}

var _ error = BlockRequestValidationError{}

var _ interface {
	Field() string
	Reason() string
	Key() bool
	Cause() error
	ErrorName() string
} = BlockRequestValidationError{}
