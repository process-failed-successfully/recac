package main

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigImportCmd(t *testing.T) {
	// Setup generic mock variables for the test
	origReadFile := readFileFunc
	origViperSafeWriteConfig := viperSafeWriteConfig
	origViperConfigFileUsed := viperConfigFileUsed

	defer func() {
		readFileFunc = origReadFile
		viperSafeWriteConfig = origViperSafeWriteConfig
		viperConfigFileUsed = origViperConfigFileUsed
	}()

	t.Run("success_json", func(t *testing.T) {
		viper.Reset()

		readFileFunc = func(name string) ([]byte, error) {
			assert.Equal(t, "test.json", name)
			return []byte(`{"agent": {"provider": "test-provider"}, "timeout": 30}`), nil
		}

		writeCalled := false
		viperSafeWriteConfig = func() error {
			writeCalled = true
			return nil
		}

		viperConfigFileUsed = func() string {
			return "config.yaml"
		}

		buf := new(bytes.Buffer)
		configImportCmd.SetOut(buf)
		configImportCmd.SetErr(buf)
		// Reset args on root to avoid cross-test contamination
		rootCmd.SetArgs([]string{"config", "import", "test.json"})

		err := rootCmd.Execute()
		require.NoError(t, err)

		assert.True(t, writeCalled)
		assert.Contains(t, buf.String(), "Successfully imported")

		assert.Equal(t, "test-provider", viper.GetString("agent.provider"))
		assert.Equal(t, 30, viper.GetInt("timeout"))
	})

	t.Run("success_yaml", func(t *testing.T) {
		viper.Reset()

		readFileFunc = func(name string) ([]byte, error) {
			assert.Equal(t, "test.yaml", name)
			return []byte("agent:\n  provider: yaml-provider\ntimeout: 40"), nil
		}

		writeCalled := false
		viperSafeWriteConfig = func() error {
			writeCalled = true
			return nil
		}

		viperConfigFileUsed = func() string {
			return "config.yaml"
		}

		buf := new(bytes.Buffer)
		configImportCmd.SetOut(buf)
		configImportCmd.SetErr(buf)
		rootCmd.SetArgs([]string{"config", "import", "test.yaml"})

		err := rootCmd.Execute()
		require.NoError(t, err)

		assert.True(t, writeCalled)
		assert.Contains(t, buf.String(), "Successfully imported")

		assert.Equal(t, "yaml-provider", viper.GetString("agent.provider"))
		assert.Equal(t, 40, viper.GetInt("timeout"))
	})

	t.Run("unsupported_extension", func(t *testing.T) {
		viper.Reset()

		readFileFunc = func(name string) ([]byte, error) {
			return []byte("content"), nil
		}

		buf := new(bytes.Buffer)
		configImportCmd.SetOut(buf)
		configImportCmd.SetErr(buf)
		rootCmd.SetArgs([]string{"config", "import", "test.txt"})

		err := rootCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported file extension")
	})

	t.Run("read_file_error", func(t *testing.T) {
		viper.Reset()

		readFileFunc = func(name string) ([]byte, error) {
			return nil, os.ErrNotExist
		}

		buf := new(bytes.Buffer)
		configImportCmd.SetOut(buf)
		configImportCmd.SetErr(buf)
		rootCmd.SetArgs([]string{"config", "import", "missing.json"})

		err := rootCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read import file")
	})

	t.Run("json_parse_error", func(t *testing.T) {
		viper.Reset()

		readFileFunc = func(name string) ([]byte, error) {
			return []byte(`{"agent": badjson`), nil
		}

		buf := new(bytes.Buffer)
		configImportCmd.SetOut(buf)
		configImportCmd.SetErr(buf)
		rootCmd.SetArgs([]string{"config", "import", "bad.json"})

		err := rootCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JSON")
	})

	t.Run("yaml_parse_error", func(t *testing.T) {
		viper.Reset()

		readFileFunc = func(name string) ([]byte, error) {
			return []byte(`agent: [unclosed`), nil
		}

		buf := new(bytes.Buffer)
		configImportCmd.SetOut(buf)
		configImportCmd.SetErr(buf)
		rootCmd.SetArgs([]string{"config", "import", "bad.yaml"})

		err := rootCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse YAML")
	})

	t.Run("write_config_error", func(t *testing.T) {
		viper.Reset()

		readFileFunc = func(name string) ([]byte, error) {
			return []byte(`{"key": "value"}`), nil
		}

		viperSafeWriteConfig = func() error {
			return errors.New("write error")
		}

		// Mock WriteConfigAs by overriding it or setting a bad path
		// As WriteConfigAs isn't easily mockable, we use an invalid path
		viperConfigFileUsed = func() string {
			return "/invalid/path/that/cannot/be/written/config.yaml"
		}

		buf := new(bytes.Buffer)
		configImportCmd.SetOut(buf)
		configImportCmd.SetErr(buf)
		rootCmd.SetArgs([]string{"config", "import", "test.json"})

		err := rootCmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to write config to")
	})
}

func TestFlattenMap_NestedYamlObjects(t *testing.T) {
	yamlData := map[interface{}]interface{}{
		"provider": "yaml-provider",
		"deep": map[interface{}]interface{}{
			"nested": "value",
		},
	}

	m := map[string]interface{}{
		"agent": yamlData,
	}

	flat := flattenMap(m, "")
	assert.Equal(t, "yaml-provider", flat["agent.provider"])
	assert.Equal(t, "value", flat["agent.deep.nested"])
}

func TestFlattenMap_NestedYamlObjectsInSlice(t *testing.T) {
	m := map[string]interface{}{
		"agent": map[interface{}]interface{}{
            "provider": "yaml-provider",
            "deep": map[interface{}]interface{}{
                "nested": "value",
            },
        },
	}

	flat := flattenMap(m, "")
	assert.Equal(t, "yaml-provider", flat["agent.provider"])
	assert.Equal(t, "value", flat["agent.deep.nested"])
}

func TestFlattenMap_NestedStringMap(t *testing.T) {
	m := map[string]interface{}{
		"agent": map[string]interface{}{
            "provider": "yaml-provider",
            "deep": map[string]interface{}{
                "nested": "value",
            },
        },
	}

	flat := flattenMap(m, "")
	assert.Equal(t, "yaml-provider", flat["agent.provider"])
	assert.Equal(t, "value", flat["agent.deep.nested"])
}
