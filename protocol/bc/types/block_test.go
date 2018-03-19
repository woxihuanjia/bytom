package types

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"

	"github.com/bytom/protocol/bc"
	"github.com/bytom/testutil"
)

func TestMarshalBlock(t *testing.T) {
	b := &Block{
		BlockHeader: BlockHeader{
			Version: 1,
			Height:  1,
		},

		Transactions: []*Tx{
			NewTx(TxData{
				Version:        1,
				SerializedSize: uint64(44),
				Outputs: []*TxOutput{
					NewTxOutput(bc.AssetID{}, 1, nil),
				},
			}),
		}}

	got, err := json.Marshal(b)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}

	// Include start and end quote marks because json.Marshal adds them
	// to the result of Block.MarshalText.
	wantHex := ("\"03" + // serialization flags
		"01" + // version
		"01" + // block height
		"0000000000000000000000000000000000000000000000000000000000000000" + // prev block hash
		"00" + // timestamp
		"40" + // commitment extensible field length
		"0000000000000000000000000000000000000000000000000000000000000000" + // tx merkle root
		"0000000000000000000000000000000000000000000000000000000000000000" + // tx status root
		"00" + // nonce
		"00" + // bits

		"01" + // num transactions
		"07" + // tx 0, serialization flags
		"01" + // tx 0, tx version
		"00" + // tx maxtime
		"00" + // tx 0, common witness extensible string length
		"00" + // tx 0, inputs count
		"01" + // tx 0, outputs count
		"01" + // tx 0 output 0, asset version
		"23" + // tx 0, output 0, output commitment length
		"0000000000000000000000000000000000000000000000000000000000000000" + // tx 0, output 0 commitment, asset id
		"01" + // tx 0, output 0 commitment, amount
		"01" + // tx 0, output 0 commitment vm version
		"00" + // tx 0, output 0 control program
		"00\"") // tx 0, output 0 output witness

	if !bytes.Equal(got, []byte(wantHex)) {
		t.Errorf("marshaled block bytes = %s want %s", got, []byte(wantHex))
	}

	var c Block
	err = json.Unmarshal(got, &c)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}

	if !testutil.DeepEqual(*b, c) {
		t.Errorf("expected marshaled/unmarshaled block to be:\n%sgot:\n%s", spew.Sdump(*b), spew.Sdump(c))
	}

	got[7] = 'q'
	err = json.Unmarshal(got, &c)
	if err == nil {
		t.Error("unmarshaled corrupted JSON ok, wanted error")
	}
}

func TestEmptyBlock(t *testing.T) {
	block := Block{
		BlockHeader: BlockHeader{
			Version: 1,
			Height:  1,
		},
	}

	got := serialize(t, &block)
	wantHex := ("03" + // serialization flags
		"01" + // version
		"01" + // block height
		"0000000000000000000000000000000000000000000000000000000000000000" + // prev block hash
		"00" + // timestamp
		"40" + // commitment extensible field length
		"0000000000000000000000000000000000000000000000000000000000000000" + // transactions merkle root
		"0000000000000000000000000000000000000000000000000000000000000000" + // tx status hash
		"00" + // nonce
		"00" + // bits
		"00") // num transactions
	want, _ := hex.DecodeString(wantHex)
	if !bytes.Equal(got, want) {
		t.Errorf("empty block bytes = %x want %x", got, want)
	}

	got = serialize(t, &block.BlockHeader)
	wantHex = ("01" + // serialization flags
		"01" + // version
		"01" + // block height
		"0000000000000000000000000000000000000000000000000000000000000000" + // prev block hash
		"00" + // timestamp
		"40" + // commitment extensible field length
		"0000000000000000000000000000000000000000000000000000000000000000" + // transactions merkle root
		"0000000000000000000000000000000000000000000000000000000000000000" + // tx status hash
		"00" + // nonce
		"00") // bits
	want, _ = hex.DecodeString(wantHex)
	if !bytes.Equal(got, want) {
		t.Errorf("empty block header bytes = %x want %x", got, want)
	}

	wantHash := mustDecodeHash("9609d2e45760f34cbc6c6d948c3fb9b6d7b61552d9d17fdd5b7d0cb5d2e67244")
	if h := block.Hash(); h != wantHash {
		t.Errorf("got block hash %x, want %x", h.Bytes(), wantHash.Bytes())
	}

	wTime := time.Unix(0, 0).UTC()
	if got := block.Time(); got != wTime {
		t.Errorf("empty block time = %v want %v", got, wTime)
	}
}

func TestSmallBlock(t *testing.T) {
	block := Block{
		BlockHeader: BlockHeader{
			Version: 1,
			Height:  1,
		},
		Transactions: []*Tx{NewTx(TxData{Version: CurrentTransactionVersion})},
	}

	got := serialize(t, &block)
	wantHex := ("03" + // serialization flags
		"01" + // version
		"01" + // block height
		"0000000000000000000000000000000000000000000000000000000000000000" + // prev block hash
		"00" + // timestamp
		"40" + // commitment extensible field length
		"0000000000000000000000000000000000000000000000000000000000000000" + // transactions merkle root
		"0000000000000000000000000000000000000000000000000000000000000000" + // tx status hash
		"00" + // nonce
		"00" + // bits
		"01" + // num transactions
		"070100000000") // transaction
	want, _ := hex.DecodeString(wantHex)
	if !bytes.Equal(got, want) {
		t.Errorf("small block bytes = %x want %x", got, want)
	}
}
