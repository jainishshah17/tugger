package main

import (
	"testing"
)

var defaultPolicy = `
rules:
- pattern: ^jainishshah17/.*
- pattern: (.*)
  replacement: jainishshah17/$1
`

var invalidCondition = `
rules:
- pattern: ^jainishshah17/.*
  condition: Always
- pattern: ^foo/.*
  condition: bar
- pattern: (.*)
  replacement: jainishshah17/$1
`

var badRegex = `
rules:
- pattern: ^jainishsha$(.*
`

func TestPolicy_Load(t *testing.T) {
	type fields struct {
		Rules []*Pattern
	}
	type args struct {
		in []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "happy",
			args: args{
				in: []byte(defaultPolicy),
			},
			wantErr: false,
		},
		{
			name: "parse error",
			args: args{
				in: []byte(`not yaml`),
			},
			wantErr: true,
		},
		{
			name: "invalid condition",
			args: args{
				in: []byte(invalidCondition),
			},
			wantErr: true,
		},
		{
			name: "invalid regex",
			args: args{
				in: []byte(badRegex),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Policy{
				Rules: tt.fields.Rules,
			}
			if err := p.Load(tt.args.in); (err != nil) != tt.wantErr {
				t.Errorf("Policy.Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			t.Log(p)
		})
	}
}

func TestPolicy_MutateImage(t *testing.T) {
	tests := []struct {
		name  string
		in    []byte
		image string
		want  string
		want1 bool
	}{
		{
			name:  "happy noop",
			in:    []byte(defaultPolicy),
			image: "jainishshah17/nginx",
			want:  "jainishshah17/nginx",
			want1: true,
		},
		{
			name:  "happy mutate",
			in:    []byte(defaultPolicy),
			image: "nginx",
			want:  "jainishshah17/nginx",
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPolicy()
			p.Load(tt.in)
			got, got1 := p.MutateImage(tt.image)
			if got != tt.want {
				t.Errorf("Policy.MutateImage() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Policy.MutateImage() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestPolicy_ValidateImage(t *testing.T) {
	tests := []struct {
		name  string
		in    []byte
		image string
		want  bool
	}{
		{
			name:  "happy",
			in:    []byte(defaultPolicy),
			image: "jainishshah17/nginx",
			want:  true,
		},
		{
			name:  "deny",
			in:    []byte(defaultPolicy),
			image: "nginx",
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPolicy()
			p.Load(tt.in)
			if got := p.ValidateImage(tt.image); got != tt.want {
				t.Errorf("Policy.ValidateImage() = %v, want %v", got, tt.want)
			}
		})
	}
}
