; Adapted from references/aider/aider/queries, verified against the actual grammar. Unlike
; Aider's own query (which spans only function_declarator — the "name(params)" part, not the
; body), definitions here span the full function_definition node so EndLine reflects real
; function length (see symbol.ExtractTags's name/span capture pairing).

(struct_specifier name: (type_identifier) @name.definition.class body: (_)) @definition.class
(enum_specifier name: (type_identifier) @name.definition.type body: (_)) @definition.type
(type_definition declarator: (type_identifier) @name.definition.type) @definition.type
(function_definition declarator: (function_declarator declarator: (identifier) @name.definition.function)) @definition.function
(call_expression function: (identifier) @name.reference.call)
