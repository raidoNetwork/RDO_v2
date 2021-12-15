// Code generated by fastssz. DO NOT EDIT.
// Hash: d01850b5230b64d63739118c751d0554823e8ca4d738135fbe99e6d34cb422c5
package prototype

import (
	ssz "github.com/ferranbt/fastssz"
)

// MarshalSSZ ssz marshals the Block object
func (b *Block) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(b)
}

// MarshalSSZTo ssz marshals the Block object to a target array
func (b *Block) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf
	offset := int(212)

	// Field (0) 'Num'
	dst = ssz.MarshalUint64(dst, b.Num)

	// Field (1) 'Version'
	if len(b.Version) != 3 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, b.Version...)

	// Field (2) 'Hash'
	if len(b.Hash) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, b.Hash...)

	// Field (3) 'Parent'
	if len(b.Parent) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, b.Parent...)

	// Field (4) 'Timestamp'
	dst = ssz.MarshalUint64(dst, b.Timestamp)

	// Field (5) 'Txroot'
	if len(b.Txroot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, b.Txroot...)

	// Field (6) 'Proposer'
	if b.Proposer == nil {
		b.Proposer = new(Sign)
	}
	if dst, err = b.Proposer.MarshalSSZTo(dst); err != nil {
		return
	}

	// Offset (7) 'Approvers'
	dst = ssz.WriteOffset(dst, offset)
	offset += len(b.Approvers) * 85

	// Offset (8) 'Slashers'
	dst = ssz.WriteOffset(dst, offset)
	offset += len(b.Slashers) * 85

	// Offset (9) 'Transactions'
	dst = ssz.WriteOffset(dst, offset)
	for ii := 0; ii < len(b.Transactions); ii++ {
		offset += 4
		offset += b.Transactions[ii].SizeSSZ()
	}

	// Field (7) 'Approvers'
	if len(b.Approvers) > 128 {
		err = ssz.ErrListTooBig
		return
	}
	for ii := 0; ii < len(b.Approvers); ii++ {
		if dst, err = b.Approvers[ii].MarshalSSZTo(dst); err != nil {
			return
		}
	}

	// Field (8) 'Slashers'
	if len(b.Slashers) > 128 {
		err = ssz.ErrListTooBig
		return
	}
	for ii := 0; ii < len(b.Slashers); ii++ {
		if dst, err = b.Slashers[ii].MarshalSSZTo(dst); err != nil {
			return
		}
	}

	// Field (9) 'Transactions'
	if len(b.Transactions) > 1000 {
		err = ssz.ErrListTooBig
		return
	}
	{
		offset = 4 * len(b.Transactions)
		for ii := 0; ii < len(b.Transactions); ii++ {
			dst = ssz.WriteOffset(dst, offset)
			offset += b.Transactions[ii].SizeSSZ()
		}
	}
	for ii := 0; ii < len(b.Transactions); ii++ {
		if dst, err = b.Transactions[ii].MarshalSSZTo(dst); err != nil {
			return
		}
	}

	return
}

// UnmarshalSSZ ssz unmarshals the Block object
func (b *Block) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size < 212 {
		return ssz.ErrSize
	}

	tail := buf
	var o7, o8, o9 uint64

	// Field (0) 'Num'
	b.Num = ssz.UnmarshallUint64(buf[0:8])

	// Field (1) 'Version'
	if cap(b.Version) == 0 {
		b.Version = make([]byte, 0, len(buf[8:11]))
	}
	b.Version = append(b.Version, buf[8:11]...)

	// Field (2) 'Hash'
	if cap(b.Hash) == 0 {
		b.Hash = make([]byte, 0, len(buf[11:43]))
	}
	b.Hash = append(b.Hash, buf[11:43]...)

	// Field (3) 'Parent'
	if cap(b.Parent) == 0 {
		b.Parent = make([]byte, 0, len(buf[43:75]))
	}
	b.Parent = append(b.Parent, buf[43:75]...)

	// Field (4) 'Timestamp'
	b.Timestamp = ssz.UnmarshallUint64(buf[75:83])

	// Field (5) 'Txroot'
	if cap(b.Txroot) == 0 {
		b.Txroot = make([]byte, 0, len(buf[83:115]))
	}
	b.Txroot = append(b.Txroot, buf[83:115]...)

	// Field (6) 'Proposer'
	if b.Proposer == nil {
		b.Proposer = new(Sign)
	}
	if err = b.Proposer.UnmarshalSSZ(buf[115:200]); err != nil {
		return err
	}

	// Offset (7) 'Approvers'
	if o7 = ssz.ReadOffset(buf[200:204]); o7 > size {
		return ssz.ErrOffset
	}

	if o7 < 212 {
		return ssz.ErrInvalidVariableOffset
	}

	// Offset (8) 'Slashers'
	if o8 = ssz.ReadOffset(buf[204:208]); o8 > size || o7 > o8 {
		return ssz.ErrOffset
	}

	// Offset (9) 'Transactions'
	if o9 = ssz.ReadOffset(buf[208:212]); o9 > size || o8 > o9 {
		return ssz.ErrOffset
	}

	// Field (7) 'Approvers'
	{
		buf = tail[o7:o8]
		num, err := ssz.DivideInt2(len(buf), 85, 128)
		if err != nil {
			return err
		}
		b.Approvers = make([]*Sign, num)
		for ii := 0; ii < num; ii++ {
			if b.Approvers[ii] == nil {
				b.Approvers[ii] = new(Sign)
			}
			if err = b.Approvers[ii].UnmarshalSSZ(buf[ii*85 : (ii+1)*85]); err != nil {
				return err
			}
		}
	}

	// Field (8) 'Slashers'
	{
		buf = tail[o8:o9]
		num, err := ssz.DivideInt2(len(buf), 85, 128)
		if err != nil {
			return err
		}
		b.Slashers = make([]*Sign, num)
		for ii := 0; ii < num; ii++ {
			if b.Slashers[ii] == nil {
				b.Slashers[ii] = new(Sign)
			}
			if err = b.Slashers[ii].UnmarshalSSZ(buf[ii*85 : (ii+1)*85]); err != nil {
				return err
			}
		}
	}

	// Field (9) 'Transactions'
	{
		buf = tail[o9:]
		num, err := ssz.DecodeDynamicLength(buf, 1000)
		if err != nil {
			return err
		}
		b.Transactions = make([]*Transaction, num)
		err = ssz.UnmarshalDynamic(buf, num, func(indx int, buf []byte) (err error) {
			if b.Transactions[indx] == nil {
				b.Transactions[indx] = new(Transaction)
			}
			if err = b.Transactions[indx].UnmarshalSSZ(buf); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the Block object
func (b *Block) SizeSSZ() (size int) {
	size = 212

	// Field (7) 'Approvers'
	size += len(b.Approvers) * 85

	// Field (8) 'Slashers'
	size += len(b.Slashers) * 85

	// Field (9) 'Transactions'
	for ii := 0; ii < len(b.Transactions); ii++ {
		size += 4
		size += b.Transactions[ii].SizeSSZ()
	}

	return
}

// HashTreeRoot ssz hashes the Block object
func (b *Block) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(b)
}

// HashTreeRootWith ssz hashes the Block object with a hasher
func (b *Block) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'Num'
	hh.PutUint64(b.Num)

	// Field (1) 'Version'
	if len(b.Version) != 3 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(b.Version)

	// Field (2) 'Hash'
	if len(b.Hash) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(b.Hash)

	// Field (3) 'Parent'
	if len(b.Parent) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(b.Parent)

	// Field (4) 'Timestamp'
	hh.PutUint64(b.Timestamp)

	// Field (5) 'Txroot'
	if len(b.Txroot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(b.Txroot)

	// Field (6) 'Proposer'
	if err = b.Proposer.HashTreeRootWith(hh); err != nil {
		return
	}

	// Field (7) 'Approvers'
	{
		subIndx := hh.Index()
		num := uint64(len(b.Approvers))
		if num > 128 {
			err = ssz.ErrIncorrectListSize
			return
		}
		for i := uint64(0); i < num; i++ {
			if err = b.Approvers[i].HashTreeRootWith(hh); err != nil {
				return
			}
		}
		hh.MerkleizeWithMixin(subIndx, num, 128)
	}

	// Field (8) 'Slashers'
	{
		subIndx := hh.Index()
		num := uint64(len(b.Slashers))
		if num > 128 {
			err = ssz.ErrIncorrectListSize
			return
		}
		for i := uint64(0); i < num; i++ {
			if err = b.Slashers[i].HashTreeRootWith(hh); err != nil {
				return
			}
		}
		hh.MerkleizeWithMixin(subIndx, num, 128)
	}

	// Field (9) 'Transactions'
	{
		subIndx := hh.Index()
		num := uint64(len(b.Transactions))
		if num > 1000 {
			err = ssz.ErrIncorrectListSize
			return
		}
		for i := uint64(0); i < num; i++ {
			if err = b.Transactions[i].HashTreeRootWith(hh); err != nil {
				return
			}
		}
		hh.MerkleizeWithMixin(subIndx, num, 1000)
	}

	hh.Merkleize(indx)
	return
}

// MarshalSSZ ssz marshals the Sign object
func (s *Sign) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(s)
}

// MarshalSSZTo ssz marshals the Sign object to a target array
func (s *Sign) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf

	// Field (0) 'Address'
	if len(s.Address) != 20 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, s.Address...)

	// Field (1) 'Signature'
	if len(s.Signature) != 65 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, s.Signature...)

	return
}

// UnmarshalSSZ ssz unmarshals the Sign object
func (s *Sign) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size != 85 {
		return ssz.ErrSize
	}

	// Field (0) 'Address'
	if cap(s.Address) == 0 {
		s.Address = make([]byte, 0, len(buf[0:20]))
	}
	s.Address = append(s.Address, buf[0:20]...)

	// Field (1) 'Signature'
	if cap(s.Signature) == 0 {
		s.Signature = make([]byte, 0, len(buf[20:85]))
	}
	s.Signature = append(s.Signature, buf[20:85]...)

	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the Sign object
func (s *Sign) SizeSSZ() (size int) {
	size = 85
	return
}

// HashTreeRoot ssz hashes the Sign object
func (s *Sign) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(s)
}

// HashTreeRootWith ssz hashes the Sign object with a hasher
func (s *Sign) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'Address'
	if len(s.Address) != 20 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(s.Address)

	// Field (1) 'Signature'
	if len(s.Signature) != 65 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(s.Signature)

	hh.Merkleize(indx)
	return
}

// MarshalSSZ ssz marshals the Transaction object
func (t *Transaction) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(t)
}

// MarshalSSZTo ssz marshals the Transaction object to a target array
func (t *Transaction) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf
	offset := int(137)

	// Field (0) 'Num'
	dst = ssz.MarshalUint64(dst, t.Num)

	// Field (1) 'Type'
	dst = ssz.MarshalUint32(dst, t.Type)

	// Field (2) 'Timestamp'
	dst = ssz.MarshalUint64(dst, t.Timestamp)

	// Field (3) 'Hash'
	if len(t.Hash) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, t.Hash...)

	// Field (4) 'Fee'
	dst = ssz.MarshalUint64(dst, t.Fee)

	// Offset (5) 'Data'
	dst = ssz.WriteOffset(dst, offset)
	offset += len(t.Data)

	// Offset (6) 'Inputs'
	dst = ssz.WriteOffset(dst, offset)
	offset += len(t.Inputs) * 64

	// Offset (7) 'Outputs'
	dst = ssz.WriteOffset(dst, offset)
	for ii := 0; ii < len(t.Outputs); ii++ {
		offset += 4
		offset += t.Outputs[ii].SizeSSZ()
	}

	// Field (8) 'Signature'
	if len(t.Signature) != 65 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, t.Signature...)

	// Field (5) 'Data'
	if len(t.Data) > 1000000 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, t.Data...)

	// Field (6) 'Inputs'
	if len(t.Inputs) > 2000 {
		err = ssz.ErrListTooBig
		return
	}
	for ii := 0; ii < len(t.Inputs); ii++ {
		if dst, err = t.Inputs[ii].MarshalSSZTo(dst); err != nil {
			return
		}
	}

	// Field (7) 'Outputs'
	if len(t.Outputs) > 2000 {
		err = ssz.ErrListTooBig
		return
	}
	{
		offset = 4 * len(t.Outputs)
		for ii := 0; ii < len(t.Outputs); ii++ {
			dst = ssz.WriteOffset(dst, offset)
			offset += t.Outputs[ii].SizeSSZ()
		}
	}
	for ii := 0; ii < len(t.Outputs); ii++ {
		if dst, err = t.Outputs[ii].MarshalSSZTo(dst); err != nil {
			return
		}
	}

	return
}

// UnmarshalSSZ ssz unmarshals the Transaction object
func (t *Transaction) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size < 137 {
		return ssz.ErrSize
	}

	tail := buf
	var o5, o6, o7 uint64

	// Field (0) 'Num'
	t.Num = ssz.UnmarshallUint64(buf[0:8])

	// Field (1) 'Type'
	t.Type = ssz.UnmarshallUint32(buf[8:12])

	// Field (2) 'Timestamp'
	t.Timestamp = ssz.UnmarshallUint64(buf[12:20])

	// Field (3) 'Hash'
	if cap(t.Hash) == 0 {
		t.Hash = make([]byte, 0, len(buf[20:52]))
	}
	t.Hash = append(t.Hash, buf[20:52]...)

	// Field (4) 'Fee'
	t.Fee = ssz.UnmarshallUint64(buf[52:60])

	// Offset (5) 'Data'
	if o5 = ssz.ReadOffset(buf[60:64]); o5 > size {
		return ssz.ErrOffset
	}

	if o5 < 137 {
		return ssz.ErrInvalidVariableOffset
	}

	// Offset (6) 'Inputs'
	if o6 = ssz.ReadOffset(buf[64:68]); o6 > size || o5 > o6 {
		return ssz.ErrOffset
	}

	// Offset (7) 'Outputs'
	if o7 = ssz.ReadOffset(buf[68:72]); o7 > size || o6 > o7 {
		return ssz.ErrOffset
	}

	// Field (8) 'Signature'
	if cap(t.Signature) == 0 {
		t.Signature = make([]byte, 0, len(buf[72:137]))
	}
	t.Signature = append(t.Signature, buf[72:137]...)

	// Field (5) 'Data'
	{
		buf = tail[o5:o6]
		if len(buf) > 1000000 {
			return ssz.ErrBytesLength
		}
		if cap(t.Data) == 0 {
			t.Data = make([]byte, 0, len(buf))
		}
		t.Data = append(t.Data, buf...)
	}

	// Field (6) 'Inputs'
	{
		buf = tail[o6:o7]
		num, err := ssz.DivideInt2(len(buf), 64, 2000)
		if err != nil {
			return err
		}
		t.Inputs = make([]*TxInput, num)
		for ii := 0; ii < num; ii++ {
			if t.Inputs[ii] == nil {
				t.Inputs[ii] = new(TxInput)
			}
			if err = t.Inputs[ii].UnmarshalSSZ(buf[ii*64 : (ii+1)*64]); err != nil {
				return err
			}
		}
	}

	// Field (7) 'Outputs'
	{
		buf = tail[o7:]
		num, err := ssz.DecodeDynamicLength(buf, 2000)
		if err != nil {
			return err
		}
		t.Outputs = make([]*TxOutput, num)
		err = ssz.UnmarshalDynamic(buf, num, func(indx int, buf []byte) (err error) {
			if t.Outputs[indx] == nil {
				t.Outputs[indx] = new(TxOutput)
			}
			if err = t.Outputs[indx].UnmarshalSSZ(buf); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the Transaction object
func (t *Transaction) SizeSSZ() (size int) {
	size = 137

	// Field (5) 'Data'
	size += len(t.Data)

	// Field (6) 'Inputs'
	size += len(t.Inputs) * 64

	// Field (7) 'Outputs'
	for ii := 0; ii < len(t.Outputs); ii++ {
		size += 4
		size += t.Outputs[ii].SizeSSZ()
	}

	return
}

// HashTreeRoot ssz hashes the Transaction object
func (t *Transaction) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(t)
}

// HashTreeRootWith ssz hashes the Transaction object with a hasher
func (t *Transaction) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'Num'
	hh.PutUint64(t.Num)

	// Field (1) 'Type'
	hh.PutUint32(t.Type)

	// Field (2) 'Timestamp'
	hh.PutUint64(t.Timestamp)

	// Field (3) 'Hash'
	if len(t.Hash) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(t.Hash)

	// Field (4) 'Fee'
	hh.PutUint64(t.Fee)

	// Field (5) 'Data'
	if len(t.Data) > 1000000 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(t.Data)

	// Field (6) 'Inputs'
	{
		subIndx := hh.Index()
		num := uint64(len(t.Inputs))
		if num > 2000 {
			err = ssz.ErrIncorrectListSize
			return
		}
		for i := uint64(0); i < num; i++ {
			if err = t.Inputs[i].HashTreeRootWith(hh); err != nil {
				return
			}
		}
		hh.MerkleizeWithMixin(subIndx, num, 2000)
	}

	// Field (7) 'Outputs'
	{
		subIndx := hh.Index()
		num := uint64(len(t.Outputs))
		if num > 2000 {
			err = ssz.ErrIncorrectListSize
			return
		}
		for i := uint64(0); i < num; i++ {
			if err = t.Outputs[i].HashTreeRootWith(hh); err != nil {
				return
			}
		}
		hh.MerkleizeWithMixin(subIndx, num, 2000)
	}

	// Field (8) 'Signature'
	if len(t.Signature) != 65 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(t.Signature)

	hh.Merkleize(indx)
	return
}

// MarshalSSZ ssz marshals the TxInput object
func (t *TxInput) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(t)
}

// MarshalSSZTo ssz marshals the TxInput object to a target array
func (t *TxInput) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf

	// Field (0) 'Hash'
	if len(t.Hash) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, t.Hash...)

	// Field (1) 'Index'
	dst = ssz.MarshalUint32(dst, t.Index)

	// Field (2) 'Address'
	if len(t.Address) != 20 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, t.Address...)

	// Field (3) 'Amount'
	dst = ssz.MarshalUint64(dst, t.Amount)

	return
}

// UnmarshalSSZ ssz unmarshals the TxInput object
func (t *TxInput) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size != 64 {
		return ssz.ErrSize
	}

	// Field (0) 'Hash'
	if cap(t.Hash) == 0 {
		t.Hash = make([]byte, 0, len(buf[0:32]))
	}
	t.Hash = append(t.Hash, buf[0:32]...)

	// Field (1) 'Index'
	t.Index = ssz.UnmarshallUint32(buf[32:36])

	// Field (2) 'Address'
	if cap(t.Address) == 0 {
		t.Address = make([]byte, 0, len(buf[36:56]))
	}
	t.Address = append(t.Address, buf[36:56]...)

	// Field (3) 'Amount'
	t.Amount = ssz.UnmarshallUint64(buf[56:64])

	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the TxInput object
func (t *TxInput) SizeSSZ() (size int) {
	size = 64
	return
}

// HashTreeRoot ssz hashes the TxInput object
func (t *TxInput) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(t)
}

// HashTreeRootWith ssz hashes the TxInput object with a hasher
func (t *TxInput) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'Hash'
	if len(t.Hash) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(t.Hash)

	// Field (1) 'Index'
	hh.PutUint32(t.Index)

	// Field (2) 'Address'
	if len(t.Address) != 20 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(t.Address)

	// Field (3) 'Amount'
	hh.PutUint64(t.Amount)

	hh.Merkleize(indx)
	return
}

// MarshalSSZ ssz marshals the TxOutput object
func (t *TxOutput) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(t)
}

// MarshalSSZTo ssz marshals the TxOutput object to a target array
func (t *TxOutput) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf
	offset := int(32)

	// Field (0) 'Address'
	if len(t.Address) != 20 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, t.Address...)

	// Field (1) 'Amount'
	dst = ssz.MarshalUint64(dst, t.Amount)

	// Offset (2) 'Node'
	dst = ssz.WriteOffset(dst, offset)
	offset += len(t.Node)

	// Field (2) 'Node'
	if len(t.Node) > 20 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, t.Node...)

	return
}

// UnmarshalSSZ ssz unmarshals the TxOutput object
func (t *TxOutput) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size < 32 {
		return ssz.ErrSize
	}

	tail := buf
	var o2 uint64

	// Field (0) 'Address'
	if cap(t.Address) == 0 {
		t.Address = make([]byte, 0, len(buf[0:20]))
	}
	t.Address = append(t.Address, buf[0:20]...)

	// Field (1) 'Amount'
	t.Amount = ssz.UnmarshallUint64(buf[20:28])

	// Offset (2) 'Node'
	if o2 = ssz.ReadOffset(buf[28:32]); o2 > size {
		return ssz.ErrOffset
	}

	if o2 < 32 {
		return ssz.ErrInvalidVariableOffset
	}

	// Field (2) 'Node'
	{
		buf = tail[o2:]
		if len(buf) > 20 {
			return ssz.ErrBytesLength
		}
		if cap(t.Node) == 0 {
			t.Node = make([]byte, 0, len(buf))
		}
		t.Node = append(t.Node, buf...)
	}
	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the TxOutput object
func (t *TxOutput) SizeSSZ() (size int) {
	size = 32

	// Field (2) 'Node'
	size += len(t.Node)

	return
}

// HashTreeRoot ssz hashes the TxOutput object
func (t *TxOutput) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(t)
}

// HashTreeRootWith ssz hashes the TxOutput object with a hasher
func (t *TxOutput) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'Address'
	if len(t.Address) != 20 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(t.Address)

	// Field (1) 'Amount'
	hh.PutUint64(t.Amount)

	// Field (2) 'Node'
	if len(t.Node) > 20 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(t.Node)

	hh.Merkleize(indx)
	return
}
