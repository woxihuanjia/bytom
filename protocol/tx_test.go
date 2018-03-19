package protocol

import (
	"fmt"
	"testing"

	"golang.org/x/crypto/sha3"

	"github.com/bytom/crypto/ed25519"
	"github.com/bytom/protocol/bc"
	"github.com/bytom/protocol/bc/types"
	"github.com/bytom/protocol/vm"
	"github.com/bytom/protocol/vm/vmutil"
	"github.com/bytom/testutil"
)

/*func TestBadMaxIssuanceWindow(t *testing.T) {
	ctx := context.Background()
	c, b1 := newTestChain(t, time.Now())
	c.MaxIssuanceWindow = time.Second

	issueTx, _, _ := issue(t, nil, nil, 1)

	got, _, err := c.GenerateBlock(ctx, b1, state.Empty(), time.Now(), []*types.Tx{issueTx})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Transactions) != 0 {
		t.Error("expected issuance past max issuance window to be rejected")
	}
}*/

type testDest struct {
	privKey ed25519.PrivateKey
}

func newDest(t testing.TB) *testDest {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		testutil.FatalErr(t, err)
	}
	return &testDest{
		privKey: priv,
	}
}

func (d *testDest) sign(t testing.TB, tx *types.Tx, index uint32) {
	txsighash := tx.SigHash(index)
	prog, _ := vm.Assemble(fmt.Sprintf("0x%x TXSIGHASH EQUAL", txsighash.Bytes()))
	h := sha3.Sum256(prog)
	sig := ed25519.Sign(d.privKey, h[:])
	tx.Inputs[index].SetArguments([][]byte{vm.Int64Bytes(0), sig, prog})
}

func (d testDest) controlProgram() ([]byte, error) {
	pub := d.privKey.Public().(ed25519.PublicKey)
	return vmutil.P2SPMultiSigProgram([]ed25519.PublicKey{pub}, 1)
}

type testAsset struct {
	bc.AssetID
	testDest
}

func newAsset(t testing.TB) *testAsset {
	dest := newDest(t)
	cp, _ := dest.controlProgram()
	assetID := bc.ComputeAssetID(cp, 1, &bc.EmptyStringHash)

	return &testAsset{
		AssetID:  assetID,
		testDest: *dest,
	}
}

func issue(t testing.TB, asset *testAsset, dest *testDest, amount uint64) (*types.Tx, *testAsset, *testDest) {
	if asset == nil {
		asset = newAsset(t)
	}
	if dest == nil {
		dest = newDest(t)
	}
	assetCP, _ := asset.controlProgram()
	destCP, _ := dest.controlProgram()
	tx := types.NewTx(types.TxData{
		Version: 1,
		Inputs: []*types.TxInput{
			types.NewIssuanceInput([]byte{1}, amount, assetCP, nil, nil),
		},
		Outputs: []*types.TxOutput{
			types.NewTxOutput(asset.AssetID, amount, destCP),
		},
	})
	asset.sign(t, tx, 0)

	return tx, asset, dest
}
