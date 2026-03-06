package envloader

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func parseEnvFile(filename string) (map[string]string, error) {

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[string]string)

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {

		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		result[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func Load(filename string, cfg any) error {

	envMap := map[string]string{}

	if filename != "" {

		fileEnv, err := parseEnvFile(filename)
		if err != nil {
			return err
		}

		for k, v := range fileEnv {
			envMap[k] = v
		}
	}

	for _, e := range os.Environ() {

		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			continue
		}

		envMap[parts[0]] = parts[1]
	}

	val := reflect.ValueOf(cfg)

	if val.Kind() != reflect.Pointer || val.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("cfg must be pointer to struct")
	}

	v := val.Elem()
	t := v.Type()

	var missing []string

	for i := 0; i < v.NumField(); i++ {

		field := v.Field(i)
		fieldType := t.Field(i)

		key := fieldType.Tag.Get("env")

		if key == "" {
			key = fieldType.Name
		}

		value, exists := envMap[key]

		if !exists {

			def := fieldType.Tag.Get("default")

			if def != "" {
				value = def
				exists = true
			}
		}

		if !exists {

			if fieldType.Tag.Get("required") == "true" {
				missing = append(missing, key)
			}

			continue
		}

		err := setField(field, value, key)
		if err != nil {
			return err
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}

	return nil
}

func setField(field reflect.Value, value string, key string) error {

	switch field.Kind() {

	case reflect.String:
		field.SetString(value)

	case reflect.Int:

		if field.Type().PkgPath() == "time" && field.Type().Name() == "Duration" {

			d, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("invalid duration for %s", key)
			}

			field.SetInt(int64(d))
			return nil
		}

		i, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("field %s must be int", key)
		}

		field.SetInt(int64(i))

	case reflect.Bool:

		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("field %s must be bool", key)
		}

		field.SetBool(b)

	case reflect.Float64:

		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("field %s must be float", key)
		}

		field.SetFloat(f)

	default:
		return fmt.Errorf("unsupported type for field %s", key)
	}

	return nil
}
