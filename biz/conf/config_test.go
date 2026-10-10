package conf

import "testing"

func validModelLab() *ModelLab {
	return &ModelLab{
		Enabled:         true,
		SessionTTLHours: 24,
		StrategyModelID: "deepseek-flash",
		Models: []ModelLabModel{
			{ID: "deepseek-flash", Name: "DeepSeek Flash", Enabled: true, Provider: "deepseek", URL: "https://example.test", Model: "deepseek-flash", AccessKey: "key-1"},
			{ID: "glm-flash", Name: "GLM Flash", Enabled: true, Provider: "glm", URL: "https://example.test", Model: "glm-flash", AccessKey: "key-2"},
			{ID: "innospark", Name: "InnoSpark", Enabled: true, Provider: "innospark", URL: "https://example.test", Model: "innospark", AccessKey: "key-3"},
		},
	}
}

func TestValidateModelLab(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*ModelLab)
		wantErr bool
	}{
		{name: "valid"},
		{name: "fewer than three models", prepare: func(v *ModelLab) { v.Models = v.Models[:2] }, wantErr: true},
		{name: "fewer than three enabled models", prepare: func(v *ModelLab) { v.Models[2].Enabled = false }, wantErr: true},
		{name: "duplicate model id", prepare: func(v *ModelLab) { v.Models[2].ID = v.Models[1].ID }, wantErr: true},
		{name: "strategy model disabled", prepare: func(v *ModelLab) { v.Models[0].Enabled = false }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ModelLab: validModelLab()}
			if tt.prepare != nil {
				tt.prepare(cfg.ModelLab)
			}
			err := cfg.validateModelLab()
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateModelLab() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateModelLabDisabled(t *testing.T) {
	if err := (&Config{ModelLab: &ModelLab{Enabled: false}}).validateModelLab(); err != nil {
		t.Fatalf("disabled ModelLab should not require configuration: %v", err)
	}
}
