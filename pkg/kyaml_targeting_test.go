package pkg

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"io"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
	"testing"
)

//go:embed testdata/targeting-filter-input.yaml
var targetingFilterTestInputYAML string

//go:embed testdata/targeting-filter-expected.yaml
var targetingFilterTestExpectedYAML string

func TestTargetingFilter(t *testing.T) {
	formatYAML := func(r io.Reader) (string, error) {
		const formatYAMLFailureMessage = `%s: %w
======
%s
======`

		formatted := bytes.Buffer{}
		decoder := yaml.NewDecoder(r)
		encoder := yaml.NewEncoder(&formatted)
		encoder.SetIndent(2)
		for {
			var data interface{}
			if err := decoder.Decode(&data); err != nil {
				if err == io.EOF {
					break
				}
				return "", fmt.Errorf(formatYAMLFailureMessage, "failed decoding YAML", err, r)
			} else if err := encoder.Encode(data); err != nil {
				return "", fmt.Errorf(formatYAMLFailureMessage, "failed encoding struct", err, r)
			}
		}
		return formatted.String(), nil
	}

	out := bytes.Buffer{}
	pipeline := kio.Pipeline{
		Inputs: []kio.Reader{&kio.ByteReader{Reader: strings.NewReader(targetingFilterTestInputYAML)}},
		Filters: []kio.Filter{
			kio.FilterAll(
				yaml.Tee(
					SingleResourceTargeting(
						[]TargetingFilter{{APIVersion: "v1"}},
						[]TargetingFilter{},
					),
					yaml.SetAnnotation("apiVersion_v1", "yes"),
				),
			),
			kio.FilterAll(
				yaml.Tee(
					SingleResourceTargeting(
						[]TargetingFilter{{Kind: "ServiceAccount"}},
						[]TargetingFilter{},
					),
					yaml.SetAnnotation("kind_ServiceAccount", "yes"),
				),
			),
			kio.FilterAll(
				yaml.Tee(
					SingleResourceTargeting(
						[]TargetingFilter{{APIVersion: "apps/v1", Kind: "Deployment"}},
						[]TargetingFilter{},
					),
					yaml.SetAnnotation("apiVersionAndkind_apps_v1_Deployment", "yes"),
				),
			),
			kio.FilterAll(
				yaml.Tee(
					SingleResourceTargeting(
						[]TargetingFilter{{APIVersion: "apps/v1", Kind: "UnknownKind"}},
						[]TargetingFilter{},
					),
					yaml.SetAnnotation("apiVersionAndkind_apps_v1_UnknownKind", "yes"),
				),
			),
			kio.FilterAll(
				yaml.Tee(
					SingleResourceTargeting(
						[]TargetingFilter{{Name: "t1"}},
						[]TargetingFilter{},
					),
					yaml.SetAnnotation("name_t1", "yes"),
				),
			),
			kio.FilterAll(
				yaml.Tee(
					SingleResourceTargeting(
						[]TargetingFilter{{Namespace: "ns1"}},
						[]TargetingFilter{},
					),
					yaml.SetAnnotation("namespace_ns1", "yes"),
				),
			),
			kio.FilterAll(
				yaml.Tee(
					SingleResourceTargeting(
						[]TargetingFilter{{Kind: "ServiceAccount"}},
						[]TargetingFilter{{Namespace: "ns2"}},
					),
					yaml.SetAnnotation("serviceAccountsNotInNs2Namespace", "yes"),
				),
			),
			kio.FilterAll(
				yaml.Tee(
					SingleResourceTargeting(
						[]TargetingFilter{},
						[]TargetingFilter{{Namespace: "ns2"}},
					),
					yaml.SetAnnotation("notInNamespaceNs2", "yes"),
				),
			),
		},
		Outputs: []kio.Writer{kio.ByteWriter{Writer: &out, Style: yaml.LiteralStyle}},
	}
	if err := pipeline.Execute(); err != nil {
		t.Errorf("failed to execute pipeline: %v", err)
	}

	if actual, err := formatYAML(&out); err != nil {
		t.Fatalf("Failed to format YAML output: %s", err)
	} else if expected, err := formatYAML(strings.NewReader(targetingFilterTestExpectedYAML)); err != nil {
		t.Fatalf("Failed to format expected YAML output: %s", err)
	} else if strings.TrimSuffix(expected, "\n") != strings.TrimSuffix(actual, "\n") {
		edits := myers.ComputeEdits(span.URIFromPath("expected"), expected, actual)
		diff := fmt.Sprint(gotextdiff.ToUnified("expected", "actual", expected, edits))
		t.Fatalf("Incorrect output:\n===\n%s\n===", diff)
	}
}
