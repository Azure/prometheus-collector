#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require "deep_merge"
require "yaml"
#require_relative "ConfigParseErrorLogger"

#@@promCustomConfigPath = "/etc/config/settings/prometheus/prometheus-config"
#@@promCustomConfigPath = "rashmi"

class OtelConfigGenerator
  @@promCustomConfigPath = "/etc/config/settings/prometheus/prometheus-config"
  @@collectorConfigTemplatePath = "/opt/microsoft/otelcollector/collector-config-template.yml"
  #@@promCustomConfigPath = "rashmi"
  def self.generate_otelconfig
    begin
      puts "in file"
      if (File.exist?(@@promCustomConfigPath))
        promCustomConfig = YAML.load(File.read("/etc/config/settings/prometheus/prometheus-config"))
        collectorTemplate = YAML.load(File.read(@@collectorConfigTemplatePath))
        collectorConfig = collectorTemplate.deep_merge!(promCustomConfig)
        collectorConfigYaml = YAML.dump(collectorConfig)
        puts "collector config successfully generated..."
        return collectorConfigYaml
      else
        puts "Prometheus configmap doesnot exist"
        return nil
      end
    rescue => errorStr
      puts "Error generating collector config from prometheus custom config to run validator - #{errorStr}"
      return nil
    end
  end
end

#/usr/bin/ruby -r "./test.rb" -e "OtelConfigGenerator.generate_otelconfig"
