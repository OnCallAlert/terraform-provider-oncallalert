package provider

import (
	"context"
	"sort"

	"github.com/OnCallAlert/terraform-provider-oncallalert/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = (*escalationPolicyResource)(nil)
	_ resource.ResourceWithImportState = (*escalationPolicyResource)(nil)
)

type escalationPolicyResource struct{ base }

func newEscalationPolicyResource() resource.Resource { return &escalationPolicyResource{} }

type epActionModel struct {
	Type types.String            `tfsdk:"type"`
	Args map[string]types.String `tfsdk:"args"`
}

type epStepModel struct {
	ID           types.String    `tfsdk:"id"`
	DelayMinutes types.Int64     `tfsdk:"delay_minutes"`
	Actions      []epActionModel `tfsdk:"actions"`
}

type escalationPolicyModel struct {
	ID          types.String  `tfsdk:"id"`
	Name        types.String  `tfsdk:"name"`
	Description types.String  `tfsdk:"description"`
	Repeat      types.Int64   `tfsdk:"repeat"`
	Steps       []epStepModel `tfsdk:"steps"`
}

func (r *escalationPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_escalation_policy"
}

func (r *escalationPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An escalation policy and its ordered steps. Steps are notified in list order.",
		Attributes: map[string]schema.Attribute{
			"id":          idAttribute(),
			"name":        schema.StringAttribute{Required: true, Description: "Unique name."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"repeat": schema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(3),
				Description: "How many times to repeat the steps if nobody acknowledges. Default 3.",
			},
			"steps": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Steps, in escalation order.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{Computed: true, Description: "Server-assigned step ID."},
					"delay_minutes": schema.Int64Attribute{
						Required: true, Description: "Minutes to wait before escalating to the next step.",
					},
					"actions": schema.SetNestedAttribute{
						Required: true,
						Description: "Who or what to notify. `type` is a destination type such as `builtin-user` " +
							"(args `user_id`), `builtin-rotation` (`rotation_id`), `builtin-schedule` (`schedule_id`), " +
							"`builtin-slack-channel` (`slack_channel_id`) or `builtin-webhook` (`webhook_url`).",
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"type": schema.StringAttribute{Required: true},
							"args": schema.MapAttribute{Required: true, ElementType: types.StringType},
						}},
					},
				}},
			},
		},
	}
}

func (r *escalationPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan escalationPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := r.c.CreateEscalationPolicy(ctx, plan.Name.ValueString(), plan.Description.ValueString(), int(plan.Repeat.ValueInt64()))
	if err != nil {
		apiError(&resp.Diagnostics, "create escalation policy", err)
		return
	}
	// Save the ID first so a failed step is not an orphaned policy.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	plan.ID = types.StringValue(id)
	if err := r.syncSteps(ctx, plan, nil); err != nil {
		apiError(&resp.Diagnostics, "create escalation policy steps", err)
		return
	}
	r.readInto(ctx, id, &plan, &resp.State, &resp.Diagnostics)
}

func (r *escalationPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state escalationPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, state.ID.ValueString(), &state, &resp.State, &resp.Diagnostics)
}

func (r *escalationPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state escalationPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	if err := r.syncSteps(ctx, plan, state.Steps); err != nil {
		apiError(&resp.Diagnostics, "update escalation policy", err)
		return
	}
	r.readInto(ctx, plan.ID.ValueString(), &plan, &resp.State, &resp.Diagnostics)
}

func (r *escalationPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state escalationPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.DeleteAll(ctx, "escalationPolicy", state.ID.ValueString()); err != nil {
		apiError(&resp.Diagnostics, "delete escalation policy", err)
	}
}

func (r *escalationPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// syncSteps makes the server's steps match plan.Steps: it updates steps by position,
// creates extra ones, deletes leftovers, then sets the order and policy fields.
func (r *escalationPolicyResource) syncSteps(ctx context.Context, plan escalationPolicyModel, old []epStepModel) error {
	ids := make([]string, 0, len(plan.Steps))
	for i, s := range plan.Steps {
		delay, actions := int(s.DelayMinutes.ValueInt64()), toDestinations(s.Actions)
		if i < len(old) && old[i].ID.ValueString() != "" {
			if err := r.c.UpdateEscalationPolicyStep(ctx, old[i].ID.ValueString(), delay, actions); err != nil {
				return err
			}
			ids = append(ids, old[i].ID.ValueString())
			continue
		}
		id, err := r.c.CreateEscalationPolicyStep(ctx, plan.ID.ValueString(), delay, actions)
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}
	for i := len(plan.Steps); i < len(old); i++ {
		if err := r.c.DeleteEscalationPolicyStep(ctx, old[i].ID.ValueString()); err != nil {
			return err
		}
	}
	return r.c.UpdateEscalationPolicy(ctx, plan.ID.ValueString(), plan.Name.ValueString(),
		plan.Description.ValueString(), int(plan.Repeat.ValueInt64()), ids)
}

func (r *escalationPolicyResource) readInto(ctx context.Context, id string, prior *escalationPolicyModel, st *tfsdk.State, diags *diag.Diagnostics) {
	ep, err := r.c.GetEscalationPolicy(ctx, id)
	if err != nil {
		apiError(diags, "read escalation policy", err)
		return
	}
	if ep == nil {
		st.RemoveResource(ctx)
		return
	}
	m := epToModel(ep, prior)
	diags.Append(st.Set(ctx, &m)...)
}

func epToModel(ep *client.EscalationPolicy, prior *escalationPolicyModel) escalationPolicyModel {
	m := escalationPolicyModel{
		ID:          types.StringValue(ep.ID),
		Name:        types.StringValue(ep.Name),
		Description: types.StringValue(ep.Description),
		Repeat:      types.Int64Value(int64(ep.Repeat)),
	}
	steps := append([]client.EscalationPolicyStep(nil), ep.Steps...)
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].StepNumber < steps[j].StepNumber })
	for _, s := range steps {
		m.Steps = append(m.Steps, epStepModel{
			ID:           types.StringValue(s.ID),
			DelayMinutes: types.Int64Value(int64(s.DelayMinutes)),
			Actions:      fromDestinations(s.Actions),
		})
	}
	if len(m.Steps) == 0 && prior != nil && prior.Steps != nil {
		m.Steps = []epStepModel{} // keep an explicit empty list as empty, not null
	}
	return m
}

func toDestinations(in []epActionModel) []client.Destination {
	out := make([]client.Destination, 0, len(in))
	for _, a := range in {
		args := make(map[string]string, len(a.Args))
		for k, v := range a.Args {
			args[k] = v.ValueString()
		}
		out = append(out, client.Destination{Type: a.Type.ValueString(), Args: args})
	}
	return out
}

func fromDestinations(in []client.Destination) []epActionModel {
	out := make([]epActionModel, 0, len(in))
	for _, d := range in {
		args := make(map[string]types.String, len(d.Args))
		for k, v := range d.Args {
			args[k] = types.StringValue(v)
		}
		out = append(out, epActionModel{Type: types.StringValue(d.Type), Args: args})
	}
	return out
}
