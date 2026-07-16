; Adapted from references/aider/aider/queries, verified against the actual grammar. Name
; captures are positional here (no `name:` field on class_declaration/function_declaration in
; this grammar), matching Aider's own query exactly.

(class_declaration (type_identifier) @name.definition.class) @definition.class
(function_declaration (simple_identifier) @name.definition.function) @definition.function
(object_declaration (type_identifier) @name.definition.object) @definition.object
(call_expression (simple_identifier) @name.reference.call)
(call_expression (navigation_expression (navigation_suffix (simple_identifier) @name.reference.call)))
