; NOT adapted from Aider's lua-tags.scm — this grammar bundle uses a materially different Lua
; grammar than Aider's (tree-sitter-language-pack) does. Aider's query targets
; `function_declaration name: [(identifier) (dot_index_expression ...)]`, a node type that does
; not exist at all in this grammar; verified by dumping the actual parse tree instead of copying
; it, which is what caught this. Here, both `function foo() ... end` and
; `local function foo() ... end` parse as `function_statement`, with the name under a `name:`
; field that's wrapped in `function_name` for the non-local form but bare for the local form.

(function_statement name: (function_name (identifier) @name.definition.function)) @definition.function
(function_statement name: (identifier) @name.definition.function) @definition.function
(function_call prefix: (identifier) @name.reference.call)
