package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"github.com/BaimPriyatna/repro/src/core/errors"
	"github.com/BaimPriyatna/repro/src/operations/configmerge"
)

var configMergeCmd = &cobra.Command{
	Use:   "config-merge",
	Short: "Perform semantic merge of two configuration states",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flagFileA == "" || flagFileB == "" {
			return errors.New(errors.CodeInvalidInput, "both --a and --b file paths are required")
		}

		cfgA, err := loadConfigFile(flagFileA)
		if err != nil {
			return err
		}
		cfgB, err := loadConfigFile(flagFileB)
		if err != nil {
			return err
		}

		var res *configmerge.Result
		if flagStrict {
			res, err = configmerge.MergeStrict(cfgA, cfgB)
		} else {
			res, err = configmerge.Merge(cfgA, cfgB)
		}
		if err != nil {
			return err
		}

		return renderOutput(cmd, res, func() (string, error) {
			var b strings.Builder
			b.WriteString(fmt.Sprintf("Merge Status: %d merged keys, %d conflicts\n", len(res.Merged), len(res.Conflicts)))
			if len(res.Conflicts) > 0 {
				b.WriteString("Conflicts:\n")
				for _, c := range res.Conflicts {
					b.WriteString(fmt.Sprintf(" - %s (%s): %v vs %v\n", c.Path, c.Reason, c.ValueA, c.ValueB))
				}
			}
			return b.String(), nil
		})
	},
}

func loadConfigFile(path string) (configmerge.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInvalidInput, fmt.Sprintf("reading config file %q", path), err)
	}

	cfg := make(configmerge.Config)
	ext := strings.ToLower(filepath.Ext(path))

	if ext == ".toml" {
		if _, err := toml.Decode(string(data), &cfg); err != nil {
			return nil, errors.Wrap(errors.CodeInvalidInput, fmt.Sprintf("parsing TOML from %q", path), err)
		}
		return cfg, nil
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		// Try TOML as fallback if JSON fails
		if _, tomlErr := toml.Decode(string(data), &cfg); tomlErr != nil {
			return nil, errors.Wrap(errors.CodeInvalidInput, fmt.Sprintf("parsing JSON/TOML from %q", path), err)
		}
	}
	return cfg, nil
}

func init() {
	configMergeCmd.Flags().StringVar(&flagFileA, "a", "", "Path to configuration file A")
	configMergeCmd.Flags().StringVar(&flagFileB, "b", "", "Path to configuration file B")
	configMergeCmd.Flags().BoolVar(&flagStrict, "strict", false, "Reject merge if any conflict occurs")
}
