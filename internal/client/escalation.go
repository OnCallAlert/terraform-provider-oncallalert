package client

import "context"

// Destination is an escalation step action, e.g. {type: "builtin-rotation", args: {rotation_id: "..."}}.
type Destination struct {
	Type string            `json:"type"`
	Args map[string]string `json:"args"`
}

// EscalationPolicyStep is one step of an escalation policy.
type EscalationPolicyStep struct {
	ID           string        `json:"id"`
	StepNumber   int           `json:"stepNumber"`
	DelayMinutes int           `json:"delayMinutes"`
	Actions      []Destination `json:"actions"`
}

// EscalationPolicy is an escalation policy with its ordered steps.
type EscalationPolicy struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Repeat      int                    `json:"repeat"`
	Steps       []EscalationPolicyStep `json:"steps"`
}

const epFields = `id name description repeat steps { id stepNumber delayMinutes actions { type args } }`

// GetEscalationPolicy returns the policy, or nil if it does not exist.
func (c *Client) GetEscalationPolicy(ctx context.Context, id string) (*EscalationPolicy, error) {
	var out struct{ EscalationPolicy *EscalationPolicy }
	err := c.Do(ctx, `query($id:ID!){escalationPolicy(id:$id){`+epFields+`}}`, map[string]any{"id": id}, &out)
	if IsNotFound(err) {
		return nil, nil
	}
	return out.EscalationPolicy, err
}

// CreateEscalationPolicy creates a policy without steps and returns its ID.
func (c *Client) CreateEscalationPolicy(ctx context.Context, name, description string, repeat int) (string, error) {
	var out struct{ CreateEscalationPolicy struct{ ID string } }
	in := map[string]any{"name": name, "description": description, "repeat": repeat}
	err := c.Do(ctx, `mutation($in:CreateEscalationPolicyInput!){createEscalationPolicy(input:$in){id}}`, map[string]any{"in": in}, &out)
	return out.CreateEscalationPolicy.ID, err
}

// UpdateEscalationPolicy sets name, description, repeat and (if stepIDs is not nil) the step order.
func (c *Client) UpdateEscalationPolicy(ctx context.Context, id, name, description string, repeat int, stepIDs []string) error {
	in := map[string]any{"id": id, "name": name, "description": description, "repeat": repeat}
	if stepIDs != nil {
		in["stepIDs"] = stepIDs
	}
	return c.Do(ctx, `mutation($in:UpdateEscalationPolicyInput!){updateEscalationPolicy(input:$in)}`, map[string]any{"in": in}, nil)
}

// CreateEscalationPolicyStep appends a step to the policy and returns its ID.
func (c *Client) CreateEscalationPolicyStep(ctx context.Context, policyID string, delay int, actions []Destination) (string, error) {
	var out struct{ CreateEscalationPolicyStep struct{ ID string } }
	in := map[string]any{"escalationPolicyID": policyID, "delayMinutes": delay, "actions": nonNilDests(actions)}
	err := c.Do(ctx, `mutation($in:CreateEscalationPolicyStepInput!){createEscalationPolicyStep(input:$in){id}}`, map[string]any{"in": in}, &out)
	return out.CreateEscalationPolicyStep.ID, err
}

// UpdateEscalationPolicyStep replaces a step's delay and actions.
func (c *Client) UpdateEscalationPolicyStep(ctx context.Context, id string, delay int, actions []Destination) error {
	in := map[string]any{"id": id, "delayMinutes": delay, "actions": nonNilDests(actions)}
	return c.Do(ctx, `mutation($in:UpdateEscalationPolicyStepInput!){updateEscalationPolicyStep(input:$in)}`, map[string]any{"in": in}, nil)
}

// DeleteEscalationPolicyStep deletes one step. A missing step is not an error.
func (c *Client) DeleteEscalationPolicyStep(ctx context.Context, id string) error {
	err := c.Do(ctx, `mutation($id:ID!){deleteEscalationPolicyStep(id:$id)}`, map[string]any{"id": id}, nil)
	if IsNotFound(err) {
		return nil
	}
	return err
}

func nonNilDests(d []Destination) []Destination {
	if d == nil {
		return []Destination{}
	}
	for i := range d {
		if d[i].Args == nil {
			d[i].Args = map[string]string{}
		}
	}
	return d
}
