#!/usr/local/bin/ruby
# frozen_string_literal: true

require_relative "ConfigParseErrorLogger"

if (!ENV["OS_TYPE"].nil? && ENV["OS_TYPE"].downcase == "linux")
  require "re2"
end

# RE2 is not supported for windows
def isValidRegex_linux(str)
  begin
    # invalid regex example -> 'sel/\\'
    re2Regex = RE2::Regexp.new(str)
    return re2Regex.ok?
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while validating regex for target metric keep list - #{errorStr}, regular expression str - #{str}")
    return false
  end
end

def isValidRegex_windows(str)
  begin
    # invalid regex example -> 'sel/\\'
    re2Regex = Regexp.new(str)
    return true
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while validating regex for target metric keep list - #{errorStr}, regular expression str - #{str}")
    return false
  end
end

def isValidRegex(str)
  if ENV["OS_TYPE"] == "linux"
    return isValidRegex_linux(str)
  else
    return isValidRegex_windows(str)
  end
end