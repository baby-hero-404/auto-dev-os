; Adapted from references/aider/aider/queries, verified against the actual grammar. Simplified
; from Aider's version: it wraps function_declaration in class_body/protocol_body patterns to
; distinguish methods from top-level functions; the node shape (name: (simple_identifier)) is
; identical either way, so one pattern covers both, tagged "function". Call targets are
; positional (no `function:` field in this grammar), unlike most C-family languages.

(class_declaration name: (type_identifier) @name.definition.class) @definition.class
(protocol_declaration name: (type_identifier) @name.definition.interface) @definition.interface
(function_declaration name: (simple_identifier) @name.definition.function) @definition.function
(protocol_function_declaration name: (simple_identifier) @name.definition.function) @definition.function
(call_expression (simple_identifier) @name.reference.call)
