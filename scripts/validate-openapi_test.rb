# frozen_string_literal: true

require "minitest/autorun"
require "open3"
require "tmpdir"
require "yaml"

class ValidateOpenAPITest < Minitest::Test
  VALIDATOR = File.expand_path("validate-openapi.rb", __dir__)

  def test_accepts_matching_routes_and_resolved_references
    result = validate(
      openapi: openapi(paths: { "/health" => "get" }, schemas: { "Health" => schema }),
      router: 'r.GET("/health", health.Check)'
    )

    assert_predicate result, :success?
    assert_includes result.stdout, "OpenAPI validation passed"
  end

  def test_rejects_router_route_missing_from_openapi
    result = validate(
      openapi: openapi(paths: { "/health" => "get" }),
      router: "r.GET(\"/health\", health.Check)\nauthed.POST(\"/rooms/:id/join\", room.Join)"
    )

    refute_predicate result, :success?
    assert_includes result.stderr, "router operation missing from OpenAPI: POST /rooms/{id}/join"
  end

  def test_rejects_stale_openapi_operation_missing_from_router
    result = validate(
      openapi: openapi(paths: { "/health" => "get", "/removed" => "delete" }),
      router: 'r.GET("/health", health.Check)'
    )

    refute_predicate result, :success?
    assert_includes result.stderr, "OpenAPI operation missing from router: DELETE /removed"
  end

  def test_rejects_duplicate_router_operation
    result = validate(
      openapi: openapi(paths: { "/rooms/{id}" => "get" }),
      router: "r.GET(\"/rooms/:id\", room.Get)\nauthed.GET(\"/rooms/:id\", room.GetAgain)"
    )

    refute_predicate result, :success?
    assert_includes result.stderr, "duplicate router operation: GET /rooms/{id} (2 registrations)"
  end

  def test_rejects_duplicate_openapi_operation
    document = <<~YAML
      openapi: 3.1.0
      info:
        title: Test
        version: 1.0.0
      paths:
        /health:
          get:
            responses:
              "200":
                description: First
        /health:
          get:
            responses:
              "200":
                description: Duplicate
    YAML
    result = validate(openapi: document, router: 'r.GET("/health", health.Check)')

    refute_predicate result, :success?
    assert_includes result.stderr, "duplicate OpenAPI operation: GET /health (2 definitions)"
  end

  def test_rejects_broken_local_reference
    result = validate(
      openapi: openapi(paths: { "/health" => "get" }, reference: "#/components/schemas/Missing"),
      router: 'r.GET("/health", health.Check)'
    )

    refute_predicate result, :success?
    assert_includes result.stderr, "unresolved local reference: #/components/schemas/Missing"
  end

  def test_rejects_each_formatting_violation
    cases = {
      "missing final newline" => [openapi(paths: { "/health" => "get" }).chomp, "must end with a newline"],
      "tab" => [openapi(paths: { "/health" => "get" }).sub("  title:", "\ttitle:"), "tabs are not allowed"],
      "trailing whitespace" => [openapi(paths: { "/health" => "get" }).sub("openapi: 3.1.0", "openapi: 3.1.0 "), "trailing whitespace"],
      "CRLF" => [openapi(paths: { "/health" => "get" }).gsub("\n", "\r\n"), "must use LF line endings"]
    }

    cases.each do |name, (document, message)|
      result = validate(openapi: document, router: 'r.GET("/health", health.Check)')

      refute_predicate result, :success?, name
      assert_includes result.stderr, message, name
    end
  end

  private

  Result = Struct.new(:stdout, :stderr, :status) do
    def success?
      status.success?
    end
  end

  def validate(openapi:, router:)
    Dir.mktmpdir("validate-openapi-test") do |directory|
      openapi_path = File.join(directory, "openapi.yaml")
      router_path = File.join(directory, "router.go")
      File.binwrite(openapi_path, openapi)
      File.write(router_path, router)
      stdout, stderr, status = Open3.capture3("ruby", VALIDATOR, openapi_path, router_path)
      return Result.new(stdout, stderr, status)
    end
  end

  def openapi(paths:, schemas: {}, reference: nil)
    operations = paths.to_h do |path, method|
      response = { "description" => "OK" }
      response["content"] = {
        "application/json" => { "schema" => { "$ref" => reference } }
      } if reference
      [path, { method => { "responses" => { "200" => response } } }]
    end
    document = {
      "openapi" => "3.1.0",
      "info" => { "title" => "Test", "version" => "1.0.0" },
      "paths" => operations
    }
    document["components"] = { "schemas" => schemas } unless schemas.empty?
    YAML.dump(document)
  end

  def schema
    { "type" => "object", "properties" => { "status" => { "type" => "string" } } }
  end
end
