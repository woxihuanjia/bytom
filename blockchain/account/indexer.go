package account

import (
	"encoding/json"

	"github.com/bytom/blockchain/query"
	"github.com/bytom/protocol/bc"
)

const (
	//UTXOPreFix is StandardUTXOKey prefix
	UTXOPreFix = "ACU:"
	//SUTXOPrefix is ContractUTXOKey prefix
	SUTXOPrefix = "SCU:"
)

// StandardUTXOKey makes an account unspent outputs key to store
func StandardUTXOKey(id bc.Hash) []byte {
	name := id.String()
	return []byte(UTXOPreFix + name)
}

// ContractUTXOKey makes a smart contract unspent outputs key to store
func ContractUTXOKey(id bc.Hash) []byte {
	name := id.String()
	return []byte(SUTXOPrefix + name)
}

var emptyJSONObject = json.RawMessage(`{}`)

//Annotated init an annotated account object
func Annotated(a *Account) (*query.AnnotatedAccount, error) {
	aa := &query.AnnotatedAccount{
		ID:       a.ID,
		Alias:    a.Alias,
		Quorum:   a.Quorum,
		Tags:     &emptyJSONObject,
		XPubs:    a.XPubs,
		KeyIndex: a.KeyIndex,
	}

	tags, err := json.Marshal(a.Tags)
	if err != nil {
		return nil, err
	}
	if len(tags) > 0 {
		rawTags := json.RawMessage(tags)
		aa.Tags = &rawTags
	}

	return aa, nil
}
