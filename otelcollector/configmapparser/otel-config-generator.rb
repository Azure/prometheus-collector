#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
# require "deep_merge"
require "yaml"
require_relative "ConfigParseErrorLogger"

class OtelConfigGenerator
  @@promCustomConfigPath = "/etc/config/settings/prometheus/prometheus-config"
  @@collectorConfigTemplatePath = "/opt/microsoft/otelcollector/collector-config-template.yml"
  @@mergedCollectorConfig = "/opt/promCollectorConfig.yml"

  def self.replaceRelabelConfigRegex(relabelConfigs)
    begin
      relabelConfigs.each { |relabelConfig|
        action = relabelConfig["action"]
        if !action.nil? && !action.empty? # && (action.strip.casecmp("labeldrop") == 0 || action.strip.casecmp("labelkeep") == 0)
          replacement = relabelConfig["replacement"]
          if !replacement.nil? && !replacement.empty?
            relabelConfig["replacement"] = replacement.gsub("$", "$$")
          end
          regex = relabelConfig["regex"]
          if !regex.nil? && !regex.empty?
            relabelConfig["regex"] = regex.gsub("$", "$$")
          end
        end
      }
    rescue => errorStr
      ConfigParseErrorLogger.logError("otelConfigGenerator::Exception while replacing relabel config regexes - #{errorStr}")
    end
  end

  def self.generate_otelconfig
    begin
      promConfig = File.read(@@promCustomConfigPath)
      isPromCustomConfigValid = !!YAML.load(promConfig)
      if isPromCustomConfigValid == true
        promCustomConfig = YAML.load(promConfig)

        scfgs = promCustomConfig["scrape_configs"]
        puts "otelConfigGenerator::Starting to replace $ with $$ for regexes in relabel_configs and metric_relabel_configs if any "
        if !scfgs.nil? && !scfgs.empty? && scfgs.length > 0
          scfgs.each { |scfg|
            relabelConfigs = scfg["relabel_configs"]
            if !relabelConfigs.nil? && !relabelConfigs.empty? && relabelConfigs.length > 0
              replaceRelabelConfigRegex(relabelConfigs)
            end
            metricRelabelConfigs = scfg["metric_relabel_configs"]
            if !metricRelabelConfigs.nil? && !metricRelabelConfigs.empty? && metricRelabelConfigs.length > 0
              replaceRelabelConfigRegex(metricRelabelConfigs)
            end
          }
        end
        puts "otelConfigGenerator::Done replacing $ with $$ for regexes in relabel_configs and metric_relabel_configs"
        collectorTemplate = YAML.load(File.read(@@collectorConfigTemplatePath))
        collectorTemplate["receivers"]["prometheus"]["config"] = promCustomConfig
        # collectorConfig = collectorTemplate.deep_merge!(promCustomConfig)
        collectorConfigYaml = YAML.dump(collectorTemplate)
        puts "otelConfigGenerator::Collector config successfully generated..."
        File.open(@@mergedCollectorConfig, "w") { |file| file.puts collectorConfigYaml }
      else
        ConfigParseErrorLogger.logError("otelConfigGenerator::Invalid prometheus config provided in the configmap")
      end
    rescue => errorStr
      ConfigParseErrorLogger.logError("otelConfigGenerator::Error generating collector config from prometheus custom config to run validator - #{errorStr}")
    end
  end
end
