; Adapted from references/aider/aider/queries, verified against the actual grammar. Simplified
; from Aider's version: it wraps `function_item` in a `declaration_list` pattern to distinguish
; trait/impl methods (tagged "method") from top-level functions (tagged "function"); the node
; shape is identical either way here, so this tags all of them "function" — a simplification, not
; a gap, since both still surface as real definitions with accurate name/span.

(struct_item name: (type_identifier) @name.definition.class) @definition.class
(enum_item name: (type_identifier) @name.definition.class) @definition.class
(union_item name: (type_identifier) @name.definition.class) @definition.class
(trait_item name: (type_identifier) @name.definition.interface) @definition.interface
(function_item name: (identifier) @name.definition.function) @definition.function
(function_signature_item name: (identifier) @name.definition.function) @definition.function
(mod_item name: (identifier) @name.definition.module) @definition.module
(macro_definition name: (identifier) @name.definition.macro) @definition.macro
(call_expression function: (identifier) @name.reference.call)
(call_expression function: (field_expression field: (field_identifier) @name.reference.call))
(macro_invocation macro: (identifier) @name.reference.call)
