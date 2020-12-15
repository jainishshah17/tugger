package main

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

var defaultPolicy = `
rules:
- pattern: ^jainishshah17/.*
  condition: Exists
- pattern: nomatch/(.*)
  replacement: jainishshah17/$1
- pattern: (.*)
  replacement: jainishshah17/$1
  condition: Exists
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
		name    string
		in      []byte
		image   string
		want    string
		allowed bool
	}{
		{
			name:    "happy noop",
			in:      []byte(defaultPolicy),
			image:   "jainishshah17/nginx",
			want:    "jainishshah17/nginx",
			allowed: true,
		},
		{
			name:    "happy mutate",
			in:      []byte(defaultPolicy),
			image:   "nginx",
			want:    "jainishshah17/nginx",
			allowed: true,
		},
		{
			name:    "doesn't exist",
			in:      []byte(defaultPolicy),
			image:   "jainishshah17/nginx:notexist",
			want:    "jainishshah17/nginx:notexist",
			allowed: false,
		},
	}
	defer runMockRegistry()()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPolicy()
			if err != nil {
				t.Error(err)
			}
			p.Load(tt.in)
			got, allowed := p.MutateImage(tt.image)
			if got != tt.want {
				t.Errorf("Policy.MutateImage() got = %v, want %v", got, tt.want)
			}
			if allowed != tt.allowed {
				t.Errorf("Policy.MutateImage() allowed = %v, want %v", allowed, tt.allowed)
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
		{
			name:  "doesn't exist",
			in:    []byte(defaultPolicy),
			image: "jainishshah17/nginx:notexist",
			want:  false,
		},
	}
	defer runMockRegistry()()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewPolicy()
			if err != nil {
				t.Error(err)
			}
			p.Load(tt.in)
			if got := p.ValidateImage(tt.image); got != tt.want {
				t.Errorf("Policy.ValidateImage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewPolicy(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "policy*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up
	if _, err := tmpfile.Write([]byte(defaultPolicy)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}
	p, err := NewPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Load([]byte(defaultPolicy)); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		opts    []PolicyOption
		want    *Policy
		wantErr bool
	}{
		{
			name:    "happy file",
			opts:    []PolicyOption{WithConfigFile(tmpfile.Name())},
			want:    p,
			wantErr: false,
		},
		{
			name:    "missing file",
			opts:    []PolicyOption{WithConfigFile("foo bar.yaml")},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewPolicy(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}
