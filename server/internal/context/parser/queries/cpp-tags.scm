; Adapted from references/aider/aider/queries, verified against the actual grammar. Like c-tags,
; spans the full function_definition (not just the declarator) so EndLine reflects real length.
; A method inside a class_specifier's field_declaration_list is also a function_definition, just
; with a field_identifier declarator instead of a plain identifier — one pattern each covers it.

(class_specifier name: (type_identifier) @name.definition.class body: (_)) @definition.class
(struct_specifier name: (type_identifier) @name.definition.class body: (_)) @definition.class
(enum_specifier name: (type_identifier) @name.definition.type body: (_)) @definition.type
(type_definition declarator: (type_identifier) @name.definition.type) @definition.type
(function_definition declarator: (function_declarator declarator: (identifier) @name.definition.function)) @definition.function
(function_definition declarator: (function_declarator declarator: (field_identifier) @name.definition.method)) @definition.method
(call_expression function: (identifier) @name.reference.call)
(call_expression function: (field_expression field: (field_identifier) @name.reference.call))
