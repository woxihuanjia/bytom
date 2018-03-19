package types

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"

	"github.com/bytom/errors"
	"github.com/bytom/protocol/bc"
	"github.com/bytom/testutil"
)

func TestTransactionTrailingGarbage(t *testing.T) {
	const validTxHex = `0701000001012b00030a0908916133a0d64d1d973b631e226ef95338ad4a536b95635f32f0d04708a6f2a26380a094a58d1d09000101010103010203010129000000000000000000000000000000000000000000000000000000000000000080a094a58d1d01010100`

	var validTx Tx
	err := validTx.UnmarshalText([]byte(validTxHex))
	if err != nil {
		t.Fatal(err)
	}

	invalidTxHex := validTxHex + strings.Repeat("beef", 10)
	var invalidTx Tx
	err = invalidTx.UnmarshalText([]byte(invalidTxHex))
	if err == nil {
		t.Fatal("expected error with trailing garbage but got nil")
	}
}

func TestTransaction(t *testing.T) {
	issuanceScript := []byte{1}

	assetID := bc.ComputeAssetID(issuanceScript, 1, &bc.EmptyStringHash)

	cases := []struct {
		tx   *Tx
		hex  string
		hash bc.Hash
	}{
		{
			tx: NewTx(TxData{
				Version:        1,
				SerializedSize: uint64(6),
				Inputs:         nil,
				Outputs:        nil,
			}),
			hex: ("07" + // serflags
				"01" + // transaction version
				"00" + // tx maxtime
				"00" + // common witness extensible string length
				"00" + // inputs count
				"00"), // outputs count
			hash: mustDecodeHash("b28048bd60c4c13144fd34f408627d1be68f6cb4fdd34e879d6d791060ea7d60"),
		},
		{
			tx: NewTx(TxData{
				Version:        1,
				SerializedSize: uint64(105),
				Inputs: []*TxInput{
					NewIssuanceInput([]byte{10, 9, 8}, 1000000000000, issuanceScript, [][]byte{[]byte{1, 2, 3}}, nil),
				},
				Outputs: []*TxOutput{
					NewTxOutput(bc.AssetID{}, 1000000000000, []byte{1}),
				},
			}),
			hex: ("0701000001012b00030a0908916133a0d64d1d973b631e226ef95338ad4a536b95635f32f0d04708a6f2a26380a094a58d1d09000101010103010203010129000000000000000000000000000000000000000000000000000000000000000080a094a58d1d01010100"), // reference data
			hash: mustDecodeHash("7e6928130bc91e115f6ebe1fb4238d51e4155a7f9f809a36a5ebea7342ad1f63"),
		},
		{
			tx: NewTx(TxData{
				Version:        1,
				SerializedSize: uint64(174),
				Inputs: []*TxInput{
					NewSpendInput(nil, mustDecodeHash("dd385f6fe25d91d8c1bd0fa58951ad56b0c5229dcc01f61d9f9e8b9eb92d3292"), bc.AssetID{}, 1000000000000, 1, []byte{1}),
				},
				Outputs: []*TxOutput{
					NewTxOutput(assetID, 600000000000, []byte{1}),
					NewTxOutput(assetID, 400000000000, []byte{2}),
				},
			}),
			hex: ("0701000001014c014add385f6fe25d91d8c1bd0fa58951ad56b0c5229dcc01f61d9f9e8b9eb92d3292000000000000000000000000000000000000000000000000000000000000000080a094a58d1d010101010100020129916133a0d64d1d973b631e226ef95338ad4a536b95635f32f0d04708a6f2a26380e0a596bb11010101000129916133a0d64d1d973b631e226ef95338ad4a536b95635f32f0d04708a6f2a26380c0ee8ed20b01010200"), // output 1, output witness
			hash: mustDecodeHash("e89ea19ec8acb92d697c06ebf841020bb9f1d9ace983efcb47c09913cff99026"),
		},
	}
	for i, test := range cases {
		got := serialize(t, test.tx)
		want, _ := hex.DecodeString(test.hex)
		if !bytes.Equal(got, want) {
			t.Errorf("test %d: bytes = %x want %x", i, got, want)
		}
		if test.tx.ID != test.hash {
			t.Errorf("test %d: hash = %x want %x", i, test.tx.ID.Bytes(), test.hash.Bytes())
		}

		txJSON, err := json.Marshal(test.tx)
		if err != nil {
			t.Errorf("test %d: error marshaling tx to json: %s", i, err)
		}
		var txFromJSON Tx
		if err := json.Unmarshal(txJSON, &txFromJSON); err != nil {
			t.Errorf("test %d: error unmarshaling tx from json: %s", i, err)
		}
		if !testutil.DeepEqual(test.tx.TxData, txFromJSON.TxData) {
			t.Errorf("test %d: types.TxData -> json -> types.TxData: got:\n%s\nwant:\n%s", i, spew.Sdump(txFromJSON.TxData), spew.Sdump(test.tx.TxData))
		}

		tx1 := new(TxData)
		if err := tx1.UnmarshalText([]byte(test.hex)); err != nil {
			t.Errorf("test %d: unexpected err %v", i, err)
		}
		if !testutil.DeepEqual(*tx1, test.tx.TxData) {
			t.Errorf("test %d: tx1 is:\n%swant:\n%s", i, spew.Sdump(*tx1), spew.Sdump(test.tx.TxData))
		}
	}
}

func TestHasIssuance(t *testing.T) {
	cases := []struct {
		tx   *TxData
		want bool
	}{{
		tx: &TxData{
			Inputs: []*TxInput{NewIssuanceInput(nil, 0, nil, nil, nil)},
		},
		want: true,
	}, {
		tx: &TxData{
			Inputs: []*TxInput{
				NewSpendInput(nil, bc.Hash{}, bc.AssetID{}, 0, 0, nil),
				NewIssuanceInput(nil, 0, nil, nil, nil),
			},
		},
		want: true,
	}, {
		tx: &TxData{
			Inputs: []*TxInput{
				NewSpendInput(nil, bc.Hash{}, bc.AssetID{}, 0, 0, nil),
			},
		},
		want: false,
	}, {
		tx:   &TxData{},
		want: false,
	}}

	for _, c := range cases {
		got := c.tx.HasIssuance()
		if got != c.want {
			t.Errorf("HasIssuance(%+v) = %v want %v", c.tx, got, c.want)
		}
	}
}

func TestInvalidIssuance(t *testing.T) {
	hex := ("07" + // serflags
		"01" + // transaction version
		"00" + // tx maxtime
		"00" + // common witness extensible string length
		"01" + // inputs count
		"01" + // input 0, asset version
		"2b" + // input 0, input commitment length prefix
		"00" + // input 0, input commitment, "issuance" type
		"03" + // input 0, input commitment, nonce length prefix
		"0a0908" + // input 0, input commitment, nonce
		"0000000000000000000000000000000000000000000000000000000000000000" + // input 0, input commitment, WRONG asset id
		"80a094a58d1d" + // input 0, input commitment, amount
		"29" + // input 0, issuance input witness length prefix
		"03deff1d4319d67baa10a6d26c1fea9c3e8d30e33474efee1a610a9bb49d758d" + // input 0, issuance input witness, initial block
		"00" + // input 0, issuance input witness, asset definition
		"01" + // input 0, issuance input witness, vm version
		"01" + // input 0, issuance input witness, issuance program length prefix
		"01" + // input 0, issuance input witness, issuance program
		"01" + // input 0, issuance input witness, arguments count
		"03" + // input 0, issuance input witness, argument 0 length prefix
		"010203" + // input 0, issuance input witness, argument 0
		"01" + // outputs count
		"01" + // output 0, asset version
		"29" + // output 0, output commitment length
		"0000000000000000000000000000000000000000000000000000000000000000" + // output 0, output commitment, asset id
		"80a094a58d1d" + // output 0, output commitment, amount
		"01" + // output 0, output commitment, vm version
		"0101" + // output 0, output commitment, control program
		"066f7574707574" + // output 0, reference data
		"00" + // output 0, output witness
		"0869737375616e6365")
	tx := new(TxData)
	err := tx.UnmarshalText([]byte(hex))
	if errors.Root(err) != errBadAssetID {
		t.Errorf("want errBadAssetID, got %v", err)
	}
}

func BenchmarkTxWriteToTrue(b *testing.B) {
	tx := &Tx{}
	for i := 0; i < b.N; i++ {
		tx.writeTo(ioutil.Discard, 0)
	}
}

func BenchmarkTxWriteToFalse(b *testing.B) {
	tx := &Tx{}
	for i := 0; i < b.N; i++ {
		tx.writeTo(ioutil.Discard, serRequired)
	}
}

func BenchmarkTxWriteToTrue200(b *testing.B) {
	tx := &Tx{}
	for i := 0; i < 200; i++ {
		tx.Inputs = append(tx.Inputs, NewSpendInput(nil, bc.Hash{}, bc.AssetID{}, 0, 0, nil))
		tx.Outputs = append(tx.Outputs, NewTxOutput(bc.AssetID{}, 0, nil))
	}
	for i := 0; i < b.N; i++ {
		tx.writeTo(ioutil.Discard, 0)
	}
}

func BenchmarkTxWriteToFalse200(b *testing.B) {
	tx := &Tx{}
	for i := 0; i < 200; i++ {
		tx.Inputs = append(tx.Inputs, NewSpendInput(nil, bc.Hash{}, bc.AssetID{}, 0, 0, nil))
		tx.Outputs = append(tx.Outputs, NewTxOutput(bc.AssetID{}, 0, nil))
	}
	for i := 0; i < b.N; i++ {
		tx.writeTo(ioutil.Discard, serRequired)
	}
}

func BenchmarkTxInputWriteToTrue(b *testing.B) {
	input := NewSpendInput(nil, bc.Hash{}, bc.AssetID{}, 0, 0, nil)
	ew := errors.NewWriter(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		input.writeTo(ew, 0)
	}
}

func BenchmarkTxInputWriteToFalse(b *testing.B) {
	input := NewSpendInput(nil, bc.Hash{}, bc.AssetID{}, 0, 0, nil)
	ew := errors.NewWriter(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		input.writeTo(ew, serRequired)
	}
}

func BenchmarkTxOutputWriteToTrue(b *testing.B) {
	output := NewTxOutput(bc.AssetID{}, 0, nil)
	ew := errors.NewWriter(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		output.writeTo(ew, 0)
	}
}

func BenchmarkTxOutputWriteToFalse(b *testing.B) {
	output := NewTxOutput(bc.AssetID{}, 0, nil)
	ew := errors.NewWriter(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		output.writeTo(ew, serRequired)
	}
}

func BenchmarkAssetAmountWriteTo(b *testing.B) {
	aa := bc.AssetAmount{}
	for i := 0; i < b.N; i++ {
		aa.WriteTo(ioutil.Discard)
	}
}
