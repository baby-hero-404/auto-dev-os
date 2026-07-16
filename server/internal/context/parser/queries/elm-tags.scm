; Adapted from references/aider/aider/queries, verified against the actual grammar.

(value_declaration (function_declaration_left (lower_case_identifier) @name.definition.function)) @definition.function
(module_declaration name: (upper_case_qid (upper_case_identifier) @name.definition.module)) @definition.module
(type_declaration ((upper_case_identifier) @name.definition.type)) @definition.type
(function_call_expr (value_expr (value_qid) @name.reference.call))
