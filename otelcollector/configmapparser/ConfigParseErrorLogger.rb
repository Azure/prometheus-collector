#!/usr/local/bin/ruby
# frozen_string_literal: true

class ConfigParseErrorLogger
  require "json"
  require "colorize"

  def initialize
  end

  class << self
    def logError(prefix, message)
      begin
        errorMessage = "#{prefix}::error::#{message}"
        STDERR.puts errorMessage.red
      rescue => errorStr
        puts "#{prefix}::Error in ConfigParserErrorLogger::logError: #{errorStr}".red
      end
    end

    def logWarning(prefix, message)
      begin
        puts "#{prefix}::warning::#{message}".yellow
      rescue => errorStr
        puts "#{prefix}::Error in ConfigParserErrorLogger::logWarning: #{errorStr}".red
      end
    end

    def logSection(prefix, message)
      begin
        puts message.center(86, "*").cyan
      rescue => errorStr
        puts "#{prefix}::Error in ConfigParserErrorLogger::logSection: #{errorStr}".red
      end
    end

    def log(prefix, message)
      begin
        puts "#{prefix}::#{message}"
      rescue => errorStr
        puts "#{prefix}::Error in ConfigParserErrorLogger::log: #{errorStr}".red
      end
    end
  end
end
