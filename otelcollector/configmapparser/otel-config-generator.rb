#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require "deep_merge"
require "yaml"
require_relative "ConfigParseErrorLogger"

class OtelConfigGenerator
  @@promCustomConfigPath = "/etc/config/settings/prometheus/prometheus-config"
  @@collectorConfigTemplatePath = "/opt/microsoft/otelcollector/collector-config-template.yml"
  @@mergedCollectorConfig = "/opt/promCollectorConfig.yml"

  def self.generate_otelconfig
    begin
      promConfig = File.read(@@promCustomConfigPath)
      isPromCustomConfigValid = !!YAML.load(promConfig)
      if isPromCustomConfigValid == true
        promCustomConfig = YAML.load(promConfig)
        collectorTemplate = YAML.load(File.read(@@collectorConfigTemplatePath))
        collectorConfig = collectorTemplate.deep_merge!(promCustomConfig)
        collectorConfigYaml = YAML.dump(collectorConfig)
        puts "otelConfigValidator::Collector config successfully generated..."
        File.open(@@mergedCollectorConfig, "w") { |file| file.puts collectorConfigYaml }
      else
        ConfigParseErrorLogger.logError("otelConfigValidator::Invalid prometheus config provided in the configmap")
      end
    rescue => errorStr
      ConfigParseErrorLogger.logError("otelConfigValidator::otelConfigValidator::Error generating collector config from prometheus custom config to run validator - #{errorStr}")
    end
  end
end

#/usr/bin/ruby -r "./test.rb" -e "OtelConfigGenerator.generate_otelconfig"
