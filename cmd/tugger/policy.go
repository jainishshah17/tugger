package main

import (
	"fmt"
	"io/ioutil"
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

// PolicyOption options for NewPolicy()
type PolicyOption func(p *Policy) error

// Load loads a policy from YAML
func (p *Policy) Load(in []byte) error {
	if err := yaml.Unmarshal(in, p); err != nil {
		return err
	}
	if len(p.Rules) == 0 {
		return fmt.Errorf("policy rules must be non-empty slice")
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
	log.WithField("policy", string(in)).Print("loaded policy")
	return nil
}

// MutateImage transforms the image name according to the policy, or returns false if there were no matches
func (p *Policy) MutateImage(image string) (string, bool) {
	var msg string
	for _, rule := range p.Rules {
		if rule.re.MatchString(image) {
			image := image
			if rule.Replacement != "" {
				image = rule.re.ReplaceAllString(image, rule.Replacement)
			}
			if rule.Condition == "Exists" && !imageExists(image) {
				msg = fmt.Sprintf("%s does not exist in private registry", image)
				log.Debug(msg)
				continue
			}
			return image, true
		}
	}
	if msg != "" {
		log.Print(msg)
		SendSlackNotification(msg)
	}
	return image, false
}

// ValidateImage checks if an image conforms to any of the patterns in a policy without replacement
func (p *Policy) ValidateImage(image string) bool {
	for _, rule := range p.Rules {
		if rule.Replacement != "" {
			continue
		}
		if rule.Condition == "Exists" && !imageExists(image) {
			continue
		}
		if rule.re.MatchString(image) {
			return true
		}
	}
	return false
}

// NewPolicy creates a Policy
func NewPolicy(opts ...PolicyOption) (*Policy, error) {
	p := &Policy{}
	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// WithConfigFile loads a policy from a yaml file
func WithConfigFile(filename string) PolicyOption {
	return func(p *Policy) error {
		in, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}
		return p.Load(in)
	}
}
