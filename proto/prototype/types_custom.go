package prototype

// GetRealFee return tx fee according to it size
func (x *Transaction) GetRealFee() uint64 {
	return x.Fee * uint64(x.SizeSSZ())
}
