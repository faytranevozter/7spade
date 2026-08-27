#!/usr/bin/env ruby
# frozen_string_literal: true

require "set"
require "yaml"

ROOT = File.expand_path("..", __dir__)
OPENAPI_PATH = File.expand_path(ARGV.fetch(0, File.join(ROOT, "docs/openapi.yaml")))
ROUTER_PATH = File.expand_path(ARGV.fetch(1, File.join(ROOT, "services/api/internal/server/router.go")))
HTTP_METHODS = Set.new(%w[get post put patch delete options head trace]).freeze

errors = []
source = File.binread(OPENAPI_PATH)
openapi_label = OPENAPI_PATH.delete_prefix("#{ROOT}/")

errors << "#{openapi_label} must end with a newline" unless source.end_with?("\n")
source.each_line.with_index(1) do |line, number|
  errors << "#{openapi_label}:#{number}: tabs are not allowed" if line.include?("\t")
  errors << "#{openapi_label}:#{number}: trailing whitespace" if line.match?(/[ \t]+(?:\r?\n)?\z/)
end
errors << "#{openapi_label} must use LF line endings" if source.include?("\r\n")

begin
  yaml_document = Psych.parse(source)
  spec = YAML.safe_load(source, aliases: true)
rescue Psych::Exception => e
  errors << "#{openapi_label} does not parse: #{e.message}"
  yaml_document = nil
  spec = nil
end

if spec
  errors << "openapi must be 3.1.x" unless spec["openapi"].to_s.start_with?("3.1.")
  errors << "paths must be an object" unless spec["paths"].is_a?(Hash)

  refs = []
  visit = lambda do |value|
    case value
    when Hash
      value.each do |key, child|
        refs << child if key == "$ref"
        visit.call(child)
      end
    when Array
      value.each { |child| visit.call(child) }
    end
  end
  visit.call(spec)

  refs.uniq.grep(%r{\A#/}).each do |ref|
    resolved = ref.delete_prefix("#/").split("/").reduce(spec) do |node, raw_key|
      key = raw_key.gsub("~1", "/").gsub("~0", "~")
      node.is_a?(Hash) ? node[key] : nil
    end
    errors << "unresolved local reference: #{ref}" if resolved.nil?
  end

  openapi_operation_list = []
  root_mapping = yaml_document&.root
  if root_mapping.is_a?(Psych::Nodes::Mapping)
    root_pairs = root_mapping.children.each_slice(2)
    paths_node = root_pairs.find { |key, _value| key.is_a?(Psych::Nodes::Scalar) && key.value == "paths" }&.last
    if paths_node.is_a?(Psych::Nodes::Mapping)
      paths_node.children.each_slice(2) do |path_node, path_item_node|
        next unless path_node.is_a?(Psych::Nodes::Scalar) && path_item_node.is_a?(Psych::Nodes::Mapping)

        path_item_node.children.each_slice(2) do |method_node, _operation_node|
          next unless method_node.is_a?(Psych::Nodes::Scalar) && HTTP_METHODS.include?(method_node.value)

          openapi_operation_list << "#{method_node.value.upcase} #{path_node.value}"
        end
      end
    end
  end
  openapi_operation_list.tally.select { |_operation, count| count > 1 }.sort.each do |operation, count|
    errors << "duplicate OpenAPI operation: #{operation} (#{count} definitions)"
  end
  openapi_operations = openapi_operation_list.to_set

  router_operation_list = []
  router_source = File.read(ROUTER_PATH)
  route_pattern = /(r|internal|authed)\.(GET|POST|PUT|PATCH|DELETE)\("([^"]+)"/
  router_source.scan(route_pattern) do |group, method, path|
    prefix = group == "internal" ? "/internal" : ""
    normalized_path = (prefix + path).gsub(/:([A-Za-z0-9_]+)/, '{\1}')
    router_operation_list << "#{method} #{normalized_path}"
  end
  router_operation_list.tally.select { |_operation, count| count > 1 }.sort.each do |operation, count|
    errors << "duplicate router operation: #{operation} (#{count} registrations)"
  end
  router_operations = router_operation_list.to_set

  (router_operations - openapi_operations).sort.each do |operation|
    errors << "router operation missing from OpenAPI: #{operation}"
  end
  (openapi_operations - router_operations).sort.each do |operation|
    errors << "OpenAPI operation missing from router: #{operation}"
  end

  if errors.empty?
    puts "OpenAPI validation passed"
    puts "  operations: #{openapi_operations.size}"
    puts "  local references: #{refs.uniq.grep(%r{\A#/}).size}"
  end
end

unless errors.empty?
  warn "OpenAPI validation failed:"
  errors.each { |error| warn "  - #{error}" }
  exit 1
end
