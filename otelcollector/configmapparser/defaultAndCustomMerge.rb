# require 'test/unit'
require 'minitest/autorun'
require 'rubygems'
require "tomlrb"
require "yaml"

require_relative "ConfigParseErrorLogger"
# require_relative "Merger"
# require_relative './prometheus-config-merger.rb'

# require_relative '../default-prom-configs/apiserverDefault.yml'
# require_relative '../default-prom-configs/cadvisorDefaultDs.yml'
# require_relative 'test.yml'



class TestWordCounter < MiniTest::Unit::TestCase
# class TestWordCounter < Test::Unit::TestCase
    # def setup
    #     merger = Merger.new
    #   end
    def test_count_words
        # defaultConfigYaml = YAML.load(File.read("./default-prom-configs/apiserverDefault.yml"))
        # customConfigYaml = YAML.load(File.read("./default-prom-configs/cadvisorDefaultDs.yml"))
        # testFile = YAML.load(File.read("test.yml"))

        apiserverDefaultFile = "./default-prom-configs/apiserverDefault.yml"
        cadvisorDefaultFileDs = "./default-prom-configs/cadvisorDefaultDs.yml"
        merger = Minitest::Mock.new(Merger)
        merger.mergeDefaultScrapeConfigs(apiserverDefaultFile)
        # merger = Merger.new
        # output = merger.mergeDefaultScrapeConfigs(apiserverDefaultFile)
        # mergeDefaultAndCustomScrapeConfigs(customConfigYaml)
        # assert_equal testFile, mergeDefaultAndCustomScrapeConfigs(customConfigYaml)
        # puts "#{prefix}::Output: #{output}"
        puts "::Output: "
    end
end
