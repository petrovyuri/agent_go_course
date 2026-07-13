# Ссылки на документацию Eino по урокам курса

База: https://www.cloudwego.io/docs/eino/
Все ссылки сверены с актуальным сайдбаром документации. В конце каждого урока
прикладывайте блок вида «Документация по теме: <ссылка>».

---

## Модуль 1. Введение

| Тема урока | Ссылка |
|---|---|
| 1.1 Что такое LLM-агент | https://www.cloudwego.io/docs/eino/overview/ |
| 1.2 Архитектуры: workflow vs agent | https://www.cloudwego.io/docs/eino/overview/graph_or_agent/ |
| 1.3 Почему Go и Eino | https://www.cloudwego.io/docs/eino/overview/eino_open_source/ |
| 1.4 Установка окружения, выбор модели | https://www.cloudwego.io/docs/eino/quick_start/ |
| 1.5 Первый запуск: ChatModel + Ollama | https://www.cloudwego.io/docs/eino/quick_start/chapter_01_chatmodel_and_message/ |
| 1.6 Анатомия Eino: компоненты, оркестрация, аспекты | https://www.cloudwego.io/docs/eino/core_modules/ |

Модель Ollama (интеграция): https://www.cloudwego.io/docs/eino/ecosystem_integration/chat_model/

## Модуль 2. Eino Core

| Тема урока | Ссылка |
|---|---|
| 2.1 schema.Message и роли | https://www.cloudwego.io/docs/eino/quick_start/chapter_01_chatmodel_and_message/ |
| 2.2 ChatModel: Generate и Stream | https://www.cloudwego.io/docs/eino/core_modules/components/chat_model_guide/ |
| 2.3 Опции вызова (температура, top-p, max tokens) | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/call_option_capabilities/ |
| 2.4 Стриминг ответов | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/stream_programming_essentials/ |
| 2.5 Шаблоны промптов | https://www.cloudwego.io/docs/eino/core_modules/components/chat_template_guide/ |
| 2.6 Структурированный вывод | https://www.cloudwego.io/docs/eino/core_modules/components/chat_model_guide/ |
| 2.7 Практика: мини-ассистент | https://www.cloudwego.io/docs/eino/quick_start/chapter_01_chatmodel_and_message/ |

## Модуль 3. Оркестрация: графы состояний

| Тема урока | Ссылка |
|---|---|
| 3.1 Зачем оркестрация | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/ |
| 3.2 Chain | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/chain_graph_introduction/ |
| 3.3 Плагин EinoDev | https://www.cloudwego.io/docs/eino/core_modules/devops/ide_plugin_guide/ |
| 3.4 Graph: узлы, рёбра, START/END | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/chain_graph_introduction/ |
| 3.5 Lambda-узлы | https://www.cloudwego.io/docs/eino/core_modules/components/lambda_guide/ |
| 3.6 Ветвление AddBranch | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/chain_graph_introduction/ |
| 3.7 Типобезопасность NewGraph[I, O] | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/ |
| 3.8 Параллельные ветви | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/workflow_orchestration_framework/ |
| 3.9 Практика: маршрутизатор | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/chain_graph_introduction/ |

Просмотр графа в плагине: https://www.cloudwego.io/docs/eino/core_modules/devops/visual_orchestration_plugin_guide/

## Модуль 4. Mini Code: каркас агента

| Тема урока | Ссылка |
|---|---|
| 4.1 Что такое Mini Code | https://www.cloudwego.io/docs/eino/overview/ |
| 4.2 CLI-каркас (REPL) | профильной страницы в доке Eino нет (чистый Go) |
| 4.3 Граф разговора | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/chain_graph_introduction/ |
| 4.4 Маршрутизатор + EinoDev | https://www.cloudwego.io/docs/eino/core_modules/devops/visual_orchestration_plugin_guide/ |

## Модуль 5. Инструменты и tool-calling

| Тема урока | Ссылка |
|---|---|
| 5.1 Что такое tool-calling | https://www.cloudwego.io/docs/eino/core_modules/components/tools_node_guide/ |
| 5.2 tool.Info, схема параметров | https://www.cloudwego.io/docs/eino/core_modules/components/tools_node_guide/how_to_create_a_tool/ |
| 5.3 InvokableTool, utils.NewTool | https://www.cloudwego.io/docs/eino/core_modules/components/tools_node_guide/how_to_create_a_tool/ |
| 5.4 ToolsNode | https://www.cloudwego.io/docs/eino/core_modules/components/tools_node_guide/ |
| 5.5 Связка ChatModel и Tools | https://www.cloudwego.io/docs/eino/quick_start/chapter_04_tool_and_filesystem/ |
| 5.6 Несколько инструментов | https://www.cloudwego.io/docs/eino/core_modules/components/tools_node_guide/ |
| 5.7 Практика: калькулятор и словарь | https://www.cloudwego.io/docs/eino/quick_start/chapter_04_tool_and_filesystem/ |

## Модуль 6. Mini Code: работа с файлами

| Тема урока | Ссылка |
|---|---|
| 6.1 read_file | https://www.cloudwego.io/docs/eino/core_modules/components/tools_node_guide/how_to_create_a_tool/ |
| 6.2 list_dir и grep | https://www.cloudwego.io/docs/eino/core_modules/components/tools_node_guide/how_to_create_a_tool/ |
| 6.3 ToolsNode в графе | https://www.cloudwego.io/docs/eino/quick_start/chapter_04_tool_and_filesystem/ |
| 6.4 Демо: объясни код | https://www.cloudwego.io/docs/eino/quick_start/chapter_04_tool_and_filesystem/ |

## Модуль 7. ReAct: Reasoning + Acting

| Тема урока | Ссылка |
|---|---|
| 7.1 Идея ReAct | https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/ |
| 7.2 Цикл вручную (граф) | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/chain_graph_introduction/ |
| 7.3 Условие выхода из цикла | https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/ |
| 7.4 Готовый react.Agent | https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/ |
| 7.5 Лимиты шагов | https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/ |
| 7.6 Стриминг в ReAct | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/stream_programming_essentials/ |
| 7.7 Практика: агент-исследователь | https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/ |

## Модуль 8. Mini Code: думающий агент

| Тема урока | Ссылка |
|---|---|
| 8.1 ReAct-цикл в Mini Code | https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/ |
| 8.2 Условие выхода и лимит шагов | https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/ |
| 8.3 Переход на react.Agent | https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/ |
| 8.4 Демо: многошаговая задача | https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/ |

## Модуль 9. Состояние, память и надёжность

| Тема урока | Ссылка |
|---|---|
| 9.1 Состояние графа (WithGenLocalState) | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/checkpoint_interrupt/ |
| 9.2 Память диалога | https://www.cloudwego.io/docs/eino/quick_start/chapter_03_memory_and_session/ |
| 9.3 Checkpoints | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/checkpoint_interrupt/ |
| 9.4 Interrupt / Resume (HITL) | https://www.cloudwego.io/docs/eino/quick_start/chapter_07_interrupt_resume/ |
| 9.5 Ошибки узлов и ретраи | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/ |
| 9.6 Таймауты, контекст, отмена | https://www.cloudwego.io/docs/eino/quick_start/chapter_11_turnloop/ |
| 9.7 Практика: подтверждение действий | https://www.cloudwego.io/docs/eino/quick_start/chapter_07_interrupt_resume/ |

## Модуль 10. Mini Code: память, правка, подтверждения

| Тема урока | Ссылка |
|---|---|
| 10.1 Память сессии | https://www.cloudwego.io/docs/eino/quick_start/chapter_03_memory_and_session/ |
| 10.2 write_file / edit_file / run_command | https://www.cloudwego.io/docs/eino/quick_start/chapter_04_tool_and_filesystem/ |
| 10.3 Подтверждение перед записью (HITL) | https://www.cloudwego.io/docs/eino/quick_start/chapter_07_interrupt_resume/ |
| 10.4 Таймауты, отмена, checkpoints | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/checkpoint_interrupt/ |

## Модуль 11. RAG и MCP

| Тема урока | Ссылка |
|---|---|
| 11.1 Что такое RAG | https://www.cloudwego.io/docs/eino/core_modules/components/retriever_guide/ |
| 11.2 Embedding | https://www.cloudwego.io/docs/eino/core_modules/components/embedding_guide/ |
| 11.3 Indexer | https://www.cloudwego.io/docs/eino/core_modules/components/indexer_guide/ |
| 11.4 Retriever | https://www.cloudwego.io/docs/eino/core_modules/components/retriever_guide/ |
| 11.5 Сборка RAG-графа | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/chain_graph_introduction/ |
| 11.6 Agentic RAG | https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/ |
| 11.7 Протокол MCP | https://www.cloudwego.io/docs/eino/ecosystem_integration/tool/ |
| 11.8 Подключение MCP-сервера | https://www.cloudwego.io/docs/eino/ecosystem_integration/tool/ |

Прямой README MCP-компонента: https://github.com/cloudwego/eino-ext/blob/main/components/tool/mcp/README.md

## Модуль 12. Mini Code: знание проекта

| Тема урока | Ссылка |
|---|---|
| 12.1 Индексация кодовой базы | https://www.cloudwego.io/docs/eino/core_modules/components/indexer_guide/ |
| 12.2 Retriever по файлам | https://www.cloudwego.io/docs/eino/core_modules/components/retriever_guide/ |
| 12.3 RAG в агенте | https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/react_agent_manual/ |
| 12.4 MCP: filesystem/git-сервер | https://www.cloudwego.io/docs/eino/ecosystem_integration/tool/ |

Эмбеддинги Ollama (интеграция): https://www.cloudwego.io/docs/eino/ecosystem_integration/embedding/

## Модуль 13. Наблюдаемость (LangFuse)

| Тема урока | Ссылка |
|---|---|
| 13.1 Callbacks: OnStart/OnEnd/OnError | https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/callback_manual/ |
| 13.2 Создание проекта в LangFuse | https://www.cloudwego.io/docs/eino/ecosystem_integration/callbacks/ |
| 13.3 Трейсинг агента через LangFuse | https://www.cloudwego.io/docs/eino/quick_start/chapter_06_callback_and_trace/ |
| 13.4 Чтение трейсов | https://www.cloudwego.io/docs/eino/quick_start/chapter_06_callback_and_trace/ |

## Модуль 14. Mini Code: финал и прод-готовность

| Тема урока | Ссылка |
|---|---|
| 14.1 Трейсинг Mini Code (Callbacks + LangFuse) | https://www.cloudwego.io/docs/eino/quick_start/chapter_06_callback_and_trace/ |
| 14.2 Полировка: конфиг, флаги | профильной страницы в доке Eino нет (чистый Go) |
| 14.3 Терминальный интерфейс (Bubble Tea) | профильной страницы в доке Eino нет (внешняя библиотека charmbracelet/bubbletea) |
| 14.4 Итог курса, куда расти (мульти-агент) | https://www.cloudwego.io/docs/eino/core_modules/flow_integration_components/multi_agent_hosting/ |
