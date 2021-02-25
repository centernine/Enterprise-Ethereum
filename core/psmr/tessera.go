package psmr

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/private"
	"github.com/ethereum/go-ethereum/private/engine"
	"github.com/ethereum/go-ethereum/rpc"
)

type TesseraPrivateStateMetadataResolver struct {
	residentGroupByKey map[string]*PrivateStateMetadata
	privacyGroupById   map[types.PrivateStateIdentifier]*PrivateStateMetadata
}

func (t *TesseraPrivateStateMetadataResolver) ResolveForManagedParty(managedParty string) (*PrivateStateMetadata, error) {
	psm, found := t.residentGroupByKey[managedParty]
	if !found {
		return nil, fmt.Errorf("unable to find private state metadata for managed party %s", managedParty)
	}
	return psm, nil
}

func (t *TesseraPrivateStateMetadataResolver) ResolveForUserContext(ctx context.Context) (*PrivateStateMetadata, error) {
	psi, ok := ctx.Value(rpc.CtxPrivateStateIdentifier).(types.PrivateStateIdentifier)
	if !ok {
		psi = types.DefaultPrivateStateIdentifier
	}
	psm, found := t.privacyGroupById[psi]
	if !found {
		return nil, fmt.Errorf("unable to find private state for context psi %s", psi)
	}
	return psm, nil
}

func (t *TesseraPrivateStateMetadataResolver) PSIs() []types.PrivateStateIdentifier {
	psis := make([]types.PrivateStateIdentifier, 0, len(t.privacyGroupById))
	for psi := range t.privacyGroupById {
		psis = append(psis, psi)
	}
	return psis
}

func NewTesseraPrivateStateMetadataResolver() (PrivateStateMetadataResolver, error) {
	groups, err := private.P.Groups()
	if err != nil {
		return nil, err
	}
	residentGroupByKey := make(map[string]*PrivateStateMetadata)
	privacyGroupById := make(map[types.PrivateStateIdentifier]*PrivateStateMetadata)
	convertedGroups := make([]engine.PrivacyGroup, 0)
	for _, group := range groups {
		if group.Type == "RESIDENT" {
			// Resident group IDs come in base64 encoded, so revert to original ID
			decoded, err := base64.StdEncoding.DecodeString(group.PrivacyGroupId)
			if err != nil {
				return nil, err
			}
			group.PrivacyGroupId = string(decoded)
		}
		psi := types.ToPrivateStateIdentifier(group.PrivacyGroupId)
		existing, found := privacyGroupById[psi]
		if found {
			return nil, fmt.Errorf("privacy groups id clash id=%s existing.Name=%s duplicate.Name=%s", existing.ID, existing.Name, group.Name)
		}
		privacyGroupById[psi] = privacyGroupToPrivateStateMetadata(group)
		if group.Type == "RESIDENT" {
			for _, address := range group.Members {
				existing, found := residentGroupByKey[address]
				if found {
					return nil, fmt.Errorf("same address is part of two different groups: address=%s existing.Name=%s duplicate.Name=%s", address, existing.Name, group.Name)
				}
				residentGroupByKey[address] = privacyGroupToPrivateStateMetadata(group)
			}
		}
		convertedGroups = append(convertedGroups, group)
	}

	return &TesseraPrivateStateMetadataResolver{
		residentGroupByKey: residentGroupByKey,
		privacyGroupById:   privacyGroupById,
	}, nil
}

func privacyGroupToPrivateStateMetadata(group engine.PrivacyGroup) *PrivateStateMetadata {
	return &PrivateStateMetadata{
		ID:          types.PrivateStateIdentifier(group.PrivacyGroupId),
		Name:        group.Name,
		Description: group.Description,
		Type:        strTypeToPrivateStateType(group.Type),
		Addresses:   group.Members,
	}
}

func strTypeToPrivateStateType(strType string) PrivateStateType {
	if strType == "LEGACY" {
		return Legacy
	}
	if strType == "PANTHEON" {
		return Pantheon
	}
	return Resident
}
