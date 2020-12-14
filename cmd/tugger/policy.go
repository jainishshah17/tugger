package main

import (
	"fmt"
	"regexp"

	yaml "gopkg.in/yaml.v2"
)

// Pattern defines one rule in a policy
type Pattern struct {
	re          *regexp.Regexp
	Pattern     string
	Replacement string `yaml:",omitempty"`
	Condition   string `yaml:",omitempty"`
}

// Policy defines a policy to mutate image names
type Policy struct {
	Rules []*Pattern
}

// Load loads a policy from YAML
func (p *Policy) Load(in []byte) error {
	if err := yaml.Unmarshal(in, p); err != nil {
		return err
	}
	for _, rule := range p.Rules {
		var err error
		if rule.re, err = regexp.Compile(rule.Pattern); err != nil {
			return err
		}
		switch rule.Condition {
		case "":
		case "Always":
		case "Exists":
		default:
			return fmt.Errorf("condition must be null/Always (default) or Exists, not %s", rule.Condition)
		}
	}
	return nil
}

// MutateImage transforms the image name according to the policy, or returns false if there were no matches
func (p *Policy) MutateImage(image string) (string, bool) {
	for _, rule := range p.Rules {
		if rule.re.MatchString(image) {
			if rule.Replacement != "" {
				image = rule.re.ReplaceAllString(image, rule.Replacement)
			}
			return image, true
		}
	}
	return image, false
}

// ValidateImage checks if an image conforms to any of the patterns in a policy without replacement
func (p *Policy) ValidateImage(image string) bool {
	for _, rule := range p.Rules {
		if rule.Replacement == "" && rule.re.MatchString(image) {
			return true
		}
	}
	return false
}

// NewPolicy creates a Policy
func NewPolicy() *Policy {
	return &Policy{}
}
