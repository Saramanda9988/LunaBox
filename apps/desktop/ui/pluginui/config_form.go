package pluginui

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/plugin"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/components/better"
	"lunabox/desktop/ui/env"
	"lunabox/desktop/ui/theme"
)

// RenderConfigSchemaForm 为 ctx.config.define(schema) 自动渲染原生的设置表单分区。
func RenderConfigSchemaForm(c *ui.Context, t *theme.Theme, e *env.Env, pluginID string, schema *plugin.ConfigSchema) {
	if e == nil || e.Svc == nil || e.Svc.Plugin == nil || schema == nil {
		return
	}
	svc := e.Svc.Plugin
	values := svc.GetConfigValues(pluginID)

	ui.Column(c.Key("plugin-cfg-"+pluginID)).FillWidth().Gap(16).Children(func() {
		if schema.Description != "" {
			components.Text(c, schema.Description, theme.TextSM, theme.WeightNormal, t.TextMuted)
		}
		for _, field := range schema.Fields {
			f := field
			fKey := "cfg-" + pluginID + "-" + f.Key
			cur := values[f.Key]

			switch f.Type {
			case "boolean":
				checked, _ := cur.(bool)
				better.SettingSwitchRow(c, t, fKey, better.SettingSwitchRowProps{
					Label:   f.Label,
					Hint:    f.Description,
					Checked: checked,
					OnChange: func(v bool) {
						_ = svc.SetConfigValues(pluginID, map[string]any{f.Key: v})
					},
				})

			case "enum":
				valStr := strVal(cur)
				opts := make([]better.SelectOption, 0, len(f.Options))
				for _, o := range f.Options {
					opts = append(opts, better.SelectOption{Value: o.Value, Label: o.Label})
				}
				ui.Column(c.Key(fKey)).FillWidth().Gap(6).Children(func() {
					components.Text(c, f.Label, theme.TextSM, theme.WeightMedium, t.TextStrong)
					if f.Description != "" {
						components.Text(c, f.Description, theme.TextXS, theme.WeightNormal, t.TextMuted)
					}
					better.Select(c, t, fKey+"-sel", better.SelectProps{
						Value:   valStr,
						Options: opts,
						OnChange: func(v string) {
							_ = svc.SetConfigValues(pluginID, map[string]any{f.Key: v})
						},
					})
				})

			case "number":
				numVal := 0
				if fv, ok := cur.(float64); ok {
					numVal = int(math.Round(fv))
				} else if iv, ok := cur.(int); ok {
					numVal = iv
				}
				np := better.NumberInputProps{
					OnChange: func(v int) {
						_ = svc.SetConfigValues(pluginID, map[string]any{f.Key: v})
					},
				}
				if f.Min != nil {
					np.Min = better.Bound(int(*f.Min))
				}
				if f.Max != nil {
					np.Max = better.Bound(int(*f.Max))
				}
				if f.Step != nil && *f.Step > 0 {
					np.Step = int(*f.Step)
				}
				ui.Column(c.Key(fKey)).FillWidth().Gap(6).Children(func() {
					components.Text(c, f.Label, theme.TextSM, theme.WeightMedium, t.TextStrong)
					if f.Description != "" {
						components.Text(c, f.Description, theme.TextXS, theme.WeightNormal, t.TextMuted)
					}
					better.NumberInput(c, t, fKey+"-num", numVal, np)
				})

			default: // string | secret
				ui.Column(c.Key(fKey)).FillWidth().Gap(6).Children(func() {
					components.Text(c, f.Label, theme.TextSM, theme.WeightMedium, t.TextStrong)
					if f.Description != "" {
						components.Text(c, f.Description, theme.TextXS, theme.WeightNormal, t.TextMuted)
					}
					row := ui.Box(c.Key(fKey + "-wrap")).FillWidth()
					draft := ui.Local(row, "draft", func() string { return strVal(cur) })
					row.Children(func() {
						in := better.Input(c, t, fKey+"-in", draft, better.InputProps{
							Placeholder: f.Placeholder,
							Password:    f.Type == "secret",
						})
						if in.Changed() {
							_ = svc.SetConfigValues(pluginID, map[string]any{f.Key: *draft})
						}
					})
				})
			}
		}
		if len(schema.Fields) == 0 {
			components.Text(c, fmt.Sprintf("(%s)", pluginID), theme.TextXS, theme.WeightNormal, t.TextFaint)
		}
	})
}
