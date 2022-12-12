package config

import (
	"strings"

	"github.com/spf13/viper"
)

var v *viper.Viper

func NewConfig() *viper.Viper {

	if v != nil {
		return v
	}

	v = viper.GetViper()
	v.AddConfigPath("../")
	v.AddConfigPath("../../")
	v.AddConfigPath(".")
	v.SetConfigName("env")
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		panic(err)
	}

	return v

}
