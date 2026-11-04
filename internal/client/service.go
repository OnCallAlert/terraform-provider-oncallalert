package client

import (
	"context"
	"fmt"
)

// Label is a key/value pair on a service.
type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Service is an alerting service.
type Service struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	EscalationPolicyID string  `json:"escalationPolicyID"`
	Labels             []Label `json:"labels"`
}

// GetService returns the service (with labels), or nil if it does not exist.
func (c *Client) GetService(ctx context.Context, id string) (*Service, error) {
	var out struct{ Service *Service }
	err := c.Do(ctx, `query($id:ID!){service(id:$id){id name description escalationPolicyID labels{key value}}}`, map[string]any{"id": id}, &out)
	if IsNotFound(err) {
		return nil, nil
	}
	return out.Service, err
}

// CreateService creates a service and returns its ID.
func (c *Client) CreateService(ctx context.Context, name, description, epID string) (string, error) {
	var out struct{ CreateService struct{ ID string } }
	in := map[string]any{"name": name, "description": description, "escalationPolicyID": epID}
	err := c.Do(ctx, `mutation($in:CreateServiceInput!){createService(input:$in){id}}`, map[string]any{"in": in}, &out)
	return out.CreateService.ID, err
}

// UpdateService replaces the service's name, description and escalation policy.
func (c *Client) UpdateService(ctx context.Context, id, name, description, epID string) error {
	in := map[string]any{"id": id, "name": name, "description": description, "escalationPolicyID": epID}
	return c.Do(ctx, `mutation($in:UpdateServiceInput!){updateService(input:$in)}`, map[string]any{"in": in}, nil)
}

// SetServiceLabel sets a label on a service. An empty value removes the label.
func (c *Client) SetServiceLabel(ctx context.Context, serviceID, key, value string) error {
	in := map[string]any{"target": Target{ID: serviceID, Type: "service"}, "key": key, "value": value}
	return c.Do(ctx, `mutation($in:SetLabelInput!){setLabel(input:$in)}`, map[string]any{"in": in}, nil)
}

// IntegrationKey is an alert ingest key of a service.
type IntegrationKey struct {
	ID                 string  `json:"id"`
	ServiceID          string  `json:"serviceID"`
	Type               string  `json:"type"`
	Name               string  `json:"name"`
	Href               string  `json:"href"`
	ExternalSystemName *string `json:"externalSystemName"`
}

// GetIntegrationKey returns the key, or nil if it does not exist.
func (c *Client) GetIntegrationKey(ctx context.Context, id string) (*IntegrationKey, error) {
	var out struct{ IntegrationKey *IntegrationKey }
	err := c.Do(ctx, `query($id:ID!){integrationKey(id:$id){id serviceID type name href externalSystemName}}`, map[string]any{"id": id}, &out)
	if IsNotFound(err) {
		return nil, nil
	}
	return out.IntegrationKey, err
}

// CreateIntegrationKey creates a key and returns it (including its href).
func (c *Client) CreateIntegrationKey(ctx context.Context, k IntegrationKey) (*IntegrationKey, error) {
	var out struct{ CreateIntegrationKey *IntegrationKey }
	in := map[string]any{"serviceID": k.ServiceID, "type": k.Type, "name": k.Name}
	if k.ExternalSystemName != nil {
		in["externalSystemName"] = *k.ExternalSystemName
	}
	err := c.Do(ctx, `mutation($in:CreateIntegrationKeyInput!){createIntegrationKey(input:$in){id serviceID type name href externalSystemName}}`, map[string]any{"in": in}, &out)
	if err == nil && out.CreateIntegrationKey == nil {
		err = fmt.Errorf("createIntegrationKey returned no key")
	}
	return out.CreateIntegrationKey, err
}

// UpdateIntegrationKey sets the key's name and external system name.
func (c *Client) UpdateIntegrationKey(ctx context.Context, id, name string, externalSystemName *string) error {
	in := map[string]any{"id": id, "name": name}
	if externalSystemName != nil {
		in["externalSystemName"] = *externalSystemName
	} else {
		in["externalSystemName"] = ""
	}
	return c.Do(ctx, `mutation($in:UpdateIntegrationKeyInput!){updateIntegrationKey(input:$in)}`, map[string]any{"in": in}, nil)
}
