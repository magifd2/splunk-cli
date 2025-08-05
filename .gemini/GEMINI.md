# **Gemini Project Guidelines**

This document outlines the development and build rules specific to this project, intended for the Gemini agent. All development must strictly adhere to these principles to ensure the project's long-term quality, security, and maintainability.

For a detailed history of development and key decisions, please refer to the Development Log.  
For future development plans and roadmap, please refer to the Development Plan.

## **I. Core Philosophy & Overarching Principles**

*(哲学と最優先事項)*

These are the absolute, non-negotiable principles that guide all other decisions.  
(これらは他のすべての決定を導く、絶対的で交渉の余地のない原則です。)

### **I-1. Security First Principle (セキュリティ第一原則)**

* Security is the highest priority, overriding all other considerations such as functionality or performance. All code, dependencies, and configurations must be reviewed for potential security vulnerabilities before being committed.  
* Never trust user input, including environment variables. All external inputs must be validated and sanitized to prevent injection attacks.  
* Sensitive information (API keys, credentials) must never be hardcoded or stored in insecure locations. Configuration files containing sensitive information must be stored with restrictive permissions (e.g., 0600 for files, 0700 for directories).

### **I-2. Testability First Principle (テスト容易性第一の原則)**

* Prioritize practical "testability" over theoretical "beauty" in design. If a design pattern makes unit testing significantly difficult (e.g., self-registration via init()), it is forbidden.  
* Always ask, "How will I test this code?" Code that cannot be answered should not be written. Easily testable code is inherently loosely coupled and easy to understand.

## **II. Design & Implementation Principles**

*(設計と実装の原則)*

These principles govern how code should be designed and written.  
(これらはコードをどのように設計し、書くべきかを規定する原則です。)

### **II-1. Explicit is Better than Implicit (暗黙的な挙動の禁止)**

* Forbid "magical" implementations like changing global state in init() or behavior that changes merely by importing a package.  
* Dependency Injection must always be done explicitly (e.g., via function arguments). This makes code behavior traceable and easy to mock in tests.

### **II-2. Code Style and Quality (コードのスタイルと品質)**

* Adhere to standard Go formatting (gofmt) and idiomatic Go practices.  
* Keep functions concise and focused on a single responsibility.  
* Identify and address the root cause of bugs, avoiding temporary or superficial fixes.

### **II-3. Dependency Management (依存関係の管理)**

* Use Go Modules for dependency management.  
* Run go mod tidy after adding or removing dependencies to ensure go.mod and go.sum are up-to-date.  
* Regularly scan project dependencies for known vulnerabilities using make vulncheck.

## **III. Development Process & Workflow**

*(開発プロセスとワークフロー)*

These rules define the mandatory process for all development activities.  
(これらはすべての開発活動における必須のプロセスを定義するルールです。)

### **III-1. Definition of "Major Refactoring" (「大規模改修」の定義)**

Any change that meets one or more of the following criteria is considered a "Major Refactoring" and requires prior approval of a detailed development plan, including impact analysis and testing strategy.

* **Changes to Initialization Logic**: Any modification involving init() functions, global variables, or package initialization order.  
* **Changes to Core Interfaces**: Modifying the signature of a central interface like Provider.  
* **Widespread Impact**: A change that requires modifications across three or more packages.  
* **Introduction of Cross-Cutting Concerns**: Adding or changing functionality that affects multiple components, such as authentication, logging, or caching.

### **III-2. Safe Refactoring Protocol (安全なリファクタリング規約)**

Even for changes not classified as "Major Refactoring," the following protocol must be strictly followed to prevent system-wide failure.

* **III-2.1.【Establish Baseline】**: Commit the stable, fully-tested state of the code before starting. Commit message: refactor: Start refactoring X.  
* **III-2.2.【Make Minimal Changes】**: Make the smallest possible incremental change (e.g., extract one function, add one variable).  
* **III-2.3.【Test Immediately】**: Run make test immediately after the change.  
  * **On Success**: Commit the change immediately (feat: Introduce Y / refactor: Extract Z). Return to Step **III-2.2**.  
  * **On Failure**: **Discard all changes immediately (git reset --hard).** Do not attempt to fix the test by modifying other code. A test failure is a critical signal that the **design approach is flawed** and must be re-evaluated from scratch.  
* **III-2.4.【Complete】**: Achieve the final goal through a series of small, safe, and tested commits.

### **III-3. Commit Message Conventions (コミットメッセージの規約)**

* Use the Conventional Commits specification (e.g., feat:, fix:, refactor:, docs:).  
* Explain *why* a change was made, not just *what* was changed.

### **III-4. Build and Linting (ビルドとリント)**

* Always use the provided Makefile for building, testing, and cleaning the project.  
* Ensure all code passes lint checks (make lint) before committing.

## **IV. Tooling & Operational Safety**

*(ツール利用と操作の安全性)*

These rules govern the safe use of development tools and file system operations to prevent irreversible errors.  
(これらは不可逆的なエラーを防ぐため、開発ツールとファイルシステム操作の安全な利用法を規定するルールです。)

### **IV-1. Safe Code Modification Protocol (安全なコード修正手順)**

* **Problem**: The code replacement tool can be unreliable and may corrupt files.  
* **Rule**: To prevent file corruption, all code modifications must follow a "diff-and-replace" workflow.  
* **Process**:  
  * **IV-1.1.** Generate the complete, modified content of the target file into a **temporary file** (e.g., original_filename.new).  
  * **IV-1.2.** Execute a diff command between the original file and the new temporary file to verify the changes are exactly as intended.  
  * **IV-1.3.** Only after visual confirmation of the diff, replace the original file with the temporary file using a mv command.

### **IV-2. Safe File System Operations (安全なファイルシステム操作)**

* **Problem**: Accidental deletion of unintended files due to incorrect path context.  
* **Rule**: All commands that delete or modify files/directories (e.g., rm, mv) **must use absolute paths.**  
* **Reason**: This prevents catastrophic errors resulting from operating in an unexpected current working directory. Relative paths are strictly forbidden for destructive operations.

### **IV-3. Safe Git Commit Procedure (安全なGitコミット手順)**

* **Problem**: Complex commit messages can be misinterpreted by the shell, leading to incorrect or failed commits.  
* **Rule**: To prevent shell interpretation errors, commit messages **must be written to a temporary file first.**  
* **Process**:  
  * **IV-3.1.** Write the full commit message (including subject, body, and any special characters) to a temporary file (e.g., .git/COMMIT_MSG).  
  * **IV-3.2.** Use the git commit -F .git/COMMIT_MSG command to apply the message from the file. Direct command-line commits with -m are forbidden for multi-line or complex messages.

## **V. Documentation Principles**

*(ドキュメンテーションの原則)*

* **V-1. Language Policy**: The primary documentation will be in English (e.g., README.md). Other languages will be provided as auxiliary documentation with a language suffix (e.g., README.ja.md).  
* **V-2. Maintenance**: When a feature is changed or added, ensure all relevant documentation (README.md, CHANGELOG.md, etc.) is updated accordingly.