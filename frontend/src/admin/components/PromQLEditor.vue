<script setup lang="ts">
import {
  autocompletion,
  closeBrackets,
  closeBracketsKeymap,
  completionKeymap,
} from "@codemirror/autocomplete"
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands"
import {
  bracketMatching,
  HighlightStyle,
  syntaxHighlighting,
} from "@codemirror/language"
import { lintKeymap } from "@codemirror/lint"
import { EditorView, keymap, placeholder } from "@codemirror/view"
import { tags } from "@lezer/highlight"
import { PromQLExtension } from "@prometheus-io/codemirror-promql"
import { onBeforeUnmount, onMounted, useTemplateRef, watch } from "vue"

import { useSession } from "../stores/session"

const props = defineProps<{
  /**
   * datasource is the ID of the data source whose discovery proxy completes
   * names.
   */
  datasource:   string
  labelId:      string
  describedby?: string
}>()

const model = defineModel<string>({ required: true })

const host = useTemplateRef<HTMLElement>("host")
let view: EditorView | null = null

// Colors come from CSS variables, so that they follow the color scheme.
const highlight = HighlightStyle.define([
  {
    tag:   tags.keyword,
    color: "var(--sr-code-keyword)",
  },
  {
    tag:   [tags.function(tags.variableName), tags.function(tags.name)],
    color: "var(--sr-code-function)",
  },
  {
    tag:   [tags.number, tags.bool],
    color: "var(--sr-code-number)",
  },
  {
    tag:   tags.string,
    color: "var(--sr-code-string)",
  },
  {
    tag:   [tags.labelName, tags.attributeName],
    color: "var(--sr-code-label)",
  },
  {
    tag:       tags.comment,
    color:     "var(--bulma-text-weak)",
    fontStyle: "italic",
  },
  {
    tag:   tags.invalid,
    color: "var(--bulma-danger)",
  },
])

const theme = EditorView.theme({
  "&": {
    color:           "var(--bulma-text-strong)",
    backgroundColor: "var(--bulma-scheme-main)",
    border:          "1px solid var(--bulma-border)",
    borderRadius:    "var(--bulma-radius)",
  },
  "&.cm-focused": {
    outline:       "2px solid #2f6feb",
    outlineOffset: "2px",
  },
  ".cm-content": {
    minHeight:  "4.5em",
    padding:    "0.5em 0",
    fontFamily: "var(--bulma-family-code)",
    caretColor: "var(--bulma-text-strong)",
  },
  ".cm-line":    { padding: "0 0.75em" },
  ".cm-tooltip": {
    color:           "var(--bulma-text)",
    backgroundColor: "var(--bulma-scheme-main-bis)",
    border:          "1px solid var(--bulma-border)",
  },
  ".cm-tooltip-autocomplete > ul > li[aria-selected]": {
    color:           "#fff",
    backgroundColor: "#2f6feb",
  },
  ".cm-matchingBracket": { backgroundColor: "rgb(47 111 235 / 25%)" },
})

onMounted(() => {
  const promql = new PromQLExtension().setComplete({
    remote: {
      url:        `/api/admin/datasources/${props.datasource}/prometheus`,
      httpMethod: "POST",
      fetchFn:    async(input: RequestInfo | URL, init?: RequestInit) => {
        const res = await fetch(input, init)
        if (res.status === 401) {
          useSession().expire()
        }
        return res
      },
    },
  })
  view = new EditorView({
    parent:     host.value!,
    doc:        model.value,
    extensions: [
      history(),
      closeBrackets(),
      bracketMatching(),
      autocompletion(),
      keymap.of([
        ...closeBracketsKeymap,
        ...defaultKeymap,
        ...historyKeymap,
        ...completionKeymap,
        ...lintKeymap,
      ]),
      EditorView.lineWrapping,
      syntaxHighlighting(highlight),
      theme,
      placeholder("up"),
      promql.asExtension(),
      EditorView.contentAttributes.of({
        "aria-labelledby": props.labelId,
        ...props.describedby ? { "aria-describedby": props.describedby } : {},
      }),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          model.value = update.state.doc.toString()
        }
      }),
    ],
  })
})

// Outside changes, e.g. loading a panel, replace the document.
watch(model, (value) => {
  if (view && value !== view.state.doc.toString()) {
    view.dispatch({
      changes: {
        from:   0,
        to:     view.state.doc.length,
        insert: value,
      },
    })
  }
})

onBeforeUnmount(() => view?.destroy())
</script>

<template>
  <div ref="host" />
</template>
