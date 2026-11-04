package client

import "context"

// Target is a typed reference, e.g. {id: "...", type: "user"}.
type Target struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// Rotation is an on-call rotation.
type Rotation struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	TimeZone    string   `json:"timeZone"`
	Start       string   `json:"start"`
	Type        string   `json:"type"`
	ShiftLength int      `json:"shiftLength"`
	UserIDs     []string `json:"userIDs"`
}

// GetRotation returns the rotation, or nil if it does not exist.
func (c *Client) GetRotation(ctx context.Context, id string) (*Rotation, error) {
	var out struct{ Rotation *Rotation }
	err := c.Do(ctx, `query($id:ID!){rotation(id:$id){id name description timeZone start type shiftLength userIDs}}`, map[string]any{"id": id}, &out)
	if IsNotFound(err) {
		return nil, nil
	}
	return out.Rotation, err
}

func rotationInput(r Rotation) map[string]any {
	ids := r.UserIDs
	if ids == nil {
		ids = []string{}
	}
	return map[string]any{
		"name": r.Name, "description": r.Description, "timeZone": r.TimeZone,
		"start": r.Start, "type": r.Type, "shiftLength": r.ShiftLength, "userIDs": ids,
	}
}

// CreateRotation creates a rotation and returns its ID.
func (c *Client) CreateRotation(ctx context.Context, r Rotation) (string, error) {
	var out struct{ CreateRotation struct{ ID string } }
	err := c.Do(ctx, `mutation($in:CreateRotationInput!){createRotation(input:$in){id}}`, map[string]any{"in": rotationInput(r)}, &out)
	return out.CreateRotation.ID, err
}

// UpdateRotation replaces all settable fields of the rotation.
func (c *Client) UpdateRotation(ctx context.Context, r Rotation) error {
	in := rotationInput(r)
	in["id"] = r.ID
	return c.Do(ctx, `mutation($in:UpdateRotationInput!){updateRotation(input:$in)}`, map[string]any{"in": in}, nil)
}

// ScheduleRule is a time-of-day rule that puts a user or rotation on call for a schedule.
type ScheduleRule struct {
	ID            string `json:"id"`
	ScheduleID    string `json:"scheduleID"`
	Start         string `json:"start"`
	End           string `json:"end"`
	WeekdayFilter []bool `json:"weekdayFilter"`
	Target        Target `json:"target"`
}

// Schedule is an on-call schedule.
type Schedule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TimeZone    string `json:"timeZone"`
	Targets     []struct {
		Rules []ScheduleRule `json:"rules"`
	} `json:"targets"`
}

// GetSchedule returns the schedule (with its rules), or nil if it does not exist.
func (c *Client) GetSchedule(ctx context.Context, id string) (*Schedule, error) {
	var out struct{ Schedule *Schedule }
	err := c.Do(ctx, `query($id:ID!){schedule(id:$id){id name description timeZone
		targets { rules { id scheduleID start end weekdayFilter target { id type } } }}}`, map[string]any{"id": id}, &out)
	if IsNotFound(err) {
		return nil, nil
	}
	return out.Schedule, err
}

// CreateSchedule creates a schedule and returns its ID.
func (c *Client) CreateSchedule(ctx context.Context, name, description, tz string) (string, error) {
	var out struct{ CreateSchedule struct{ ID string } }
	in := map[string]any{"name": name, "description": description, "timeZone": tz}
	err := c.Do(ctx, `mutation($in:CreateScheduleInput!){createSchedule(input:$in){id}}`, map[string]any{"in": in}, &out)
	return out.CreateSchedule.ID, err
}

// UpdateSchedule replaces the schedule's name, description and time zone.
func (c *Client) UpdateSchedule(ctx context.Context, id, name, description, tz string) error {
	in := map[string]any{"id": id, "name": name, "description": description, "timeZone": tz}
	return c.Do(ctx, `mutation($in:UpdateScheduleInput!){updateSchedule(input:$in)}`, map[string]any{"in": in}, nil)
}

// GetScheduleRule finds a rule by ID inside its schedule. It returns nil if either is gone.
func (c *Client) GetScheduleRule(ctx context.Context, scheduleID, ruleID string) (*ScheduleRule, error) {
	s, err := c.GetSchedule(ctx, scheduleID)
	if err != nil || s == nil {
		return nil, err
	}
	for _, t := range s.Targets {
		for i := range t.Rules {
			if t.Rules[i].ID == ruleID {
				return &t.Rules[i], nil
			}
		}
	}
	return nil, nil
}

// CreateScheduleRule creates a rule and returns its ID.
func (c *Client) CreateScheduleRule(ctx context.Context, r ScheduleRule) (string, error) {
	var out struct{ CreateScheduleRule struct{ ID string } }
	in := map[string]any{"scheduleID": r.ScheduleID, "target": r.Target, "start": r.Start, "end": r.End, "weekdayFilter": r.WeekdayFilter}
	err := c.Do(ctx, `mutation($in:CreateScheduleRuleInput!){createScheduleRule(input:$in){id}}`, map[string]any{"in": in}, &out)
	return out.CreateScheduleRule.ID, err
}

// UpdateScheduleRule replaces the rule's times, weekdays and target.
func (c *Client) UpdateScheduleRule(ctx context.Context, r ScheduleRule) error {
	in := map[string]any{"id": r.ID, "target": r.Target, "start": r.Start, "end": r.End, "weekdayFilter": r.WeekdayFilter}
	return c.Do(ctx, `mutation($in:UpdateScheduleRuleInput!){updateScheduleRule(input:$in)}`, map[string]any{"in": in}, nil)
}

// DeleteScheduleRule deletes a rule. A missing rule is not an error.
func (c *Client) DeleteScheduleRule(ctx context.Context, id string) error {
	err := c.Do(ctx, `mutation($id:ID!){deleteScheduleRule(id:$id)}`, map[string]any{"id": id}, nil)
	if IsNotFound(err) {
		return nil
	}
	return err
}

// UserOverride adds, removes or replaces a user on a schedule for a time window.
type UserOverride struct {
	ID           string `json:"id"`
	ScheduleID   string `json:"-"`
	Start        string `json:"start"`
	End          string `json:"end"`
	AddUserID    string `json:"addUserID"`
	RemoveUserID string `json:"removeUserID"`
	Target       Target `json:"target"`
}

// GetUserOverride returns the override, or nil if it does not exist.
func (c *Client) GetUserOverride(ctx context.Context, id string) (*UserOverride, error) {
	var out struct{ UserOverride *UserOverride }
	err := c.Do(ctx, `query($id:ID!){userOverride(id:$id){id start end addUserID removeUserID target{id type}}}`, map[string]any{"id": id}, &out)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil || out.UserOverride == nil {
		return nil, err
	}
	out.UserOverride.ScheduleID = out.UserOverride.Target.ID
	return out.UserOverride, nil
}

func overrideInput(o UserOverride) map[string]any {
	in := map[string]any{"start": o.Start, "end": o.End}
	if o.AddUserID != "" {
		in["addUserID"] = o.AddUserID
	}
	if o.RemoveUserID != "" {
		in["removeUserID"] = o.RemoveUserID
	}
	return in
}

// CreateUserOverride creates an override and returns its ID.
func (c *Client) CreateUserOverride(ctx context.Context, o UserOverride) (string, error) {
	var out struct{ CreateUserOverride struct{ ID string } }
	in := overrideInput(o)
	in["scheduleID"] = o.ScheduleID
	err := c.Do(ctx, `mutation($in:CreateUserOverrideInput!){createUserOverride(input:$in){id}}`, map[string]any{"in": in}, &out)
	return out.CreateUserOverride.ID, err
}

// UpdateUserOverride replaces the override's window and users.
func (c *Client) UpdateUserOverride(ctx context.Context, o UserOverride) error {
	in := overrideInput(o)
	in["id"] = o.ID
	return c.Do(ctx, `mutation($in:UpdateUserOverrideInput!){updateUserOverride(input:$in)}`, map[string]any{"in": in}, nil)
}
