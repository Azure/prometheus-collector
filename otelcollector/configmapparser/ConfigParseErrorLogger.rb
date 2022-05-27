#!/usr/local/bin/ruby
# frozen_string_literal: true

class ConfigParseErrorLogger
  require "json"
  require "colorize"

  def initialize
  end

  class << self
    def logError(message)
      begin
        errorMessage = "config::error::" + message
        STDERR.puts errorMessage.red
      rescue => errorStr
        puts "Error in ConfigParserErrorLogger::logError: #{errorStr}".red
      end
    end

    def logWarning(message)
      begin
        warningMessage = "config::warning::" + message
        puts warningMessage.yellow
      rescue => errorStr
        puts "Error in ConfigParserErrorLogger::logWarning: #{errorStr}".red
      end
    end

    def logSection(message)
      begin
        sectionMessage = "****************" + message + "********************"
        puts warningMessage.green
      rescue => errorStr
        puts "Error in ConfigParserErrorLogger::logWarning: #{errorStr}".red
      end
    end
  end
end
