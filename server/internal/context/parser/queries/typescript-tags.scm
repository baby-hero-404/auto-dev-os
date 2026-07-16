; Adapted from references/aider/aider/queries (aider/repomap.py's own tag source, this project's
; stated inspiration — see docs/features/engineering/01-context-management.md). Node types
; verified against the actual typescript/tsx grammars (not just carried over from JavaScript or
; from Aider's query, whose Python-side grammar version can differ): class/interface/type-alias/
; abstract-class names are (type_identifier) here, not the plain (identifier) JavaScript's
; class_declaration uses — tree-sitter-typescript renames that field's node type to allow generic
; type parameters on the name position. Also, `namespace Foo {}` parses as internal_module in
; this grammar, not the bare `module` node Aider's (older, Python-grammar-targeted) query assumes
; — that pattern would have silently never matched if copied verbatim. Shared by .ts and .tsx.

(function_declaration name: (identifier) @name.definition.function) @definition.function
(function_signature name: (identifier) @name.definition.function) @definition.function
(variable_declarator name: (identifier) @name.definition.function value: (arrow_function)) @definition.function
(class_declaration name: (type_identifier) @name.definition.class) @definition.class
(abstract_class_declaration name: (type_identifier) @name.definition.class) @definition.class
(interface_declaration name: (type_identifier) @name.definition.interface) @definition.interface
(type_alias_declaration name: (type_identifier) @name.definition.type) @definition.type
(enum_declaration name: (identifier) @name.definition.enum) @definition.enum
(internal_module name: (identifier) @name.definition.module) @definition.module
(method_definition name: (property_identifier) @name.definition.method) @definition.method
(method_signature name: (property_identifier) @name.definition.method) @definition.method
(abstract_method_signature name: (property_identifier) @name.definition.method) @definition.method
(call_expression function: (identifier) @name.reference.call)
(call_expression function: (member_expression property: (property_identifier) @name.reference.call))
(new_expression constructor: (identifier) @name.reference.call)
