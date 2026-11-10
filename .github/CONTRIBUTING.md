# Contributing to Gorgeous Installer

First off, thanks for taking the time to contribute! ❤️

All types of contributions are encouraged and valued. See the [Table of Contents](#table-of-contents) for different ways to help and details about how this project handles them. Please make sure to read the relevant section before making your contribution. It will make it a lot easier for us maintainers and smooth out the experience for all involved. The community looks forward to your contributions. 🎉

> And if you like the project, but just don't have time to contribute, that's fine. There are other easy ways to support the project and show your appreciation, which we would also be very happy about:
> - ⭐ **Star the project** on [GitHub](https://github.com/Epic-Nova/Gorgeous-Installer)
> - 📖 **Refer this project** in your project's README
> - 👥🥂 **Mention the project** at local meetups and tell your friends/colleagues

## Table of Contents

- [I Have a Question](#-i-have-a-question)
- [I Want To Contribute](#-i-want-to-contribute)
    - [Reporting Bugs](#-reporting-bugs)
    - [Reporting Vulnerabilities](#%EF%B8%8F-reporting-vulnerabilities)
    - [Suggesting Enhancements](#-suggesting-enhancements)
    - [Your First Code Contribution](#-your-first-code-contribution)
    - [Improving The Documentation](#-improving-the-documentation)
- [Styleguides](#-styleguides)
    - [Naming Conventions](#%EF%B8%8F-naming-conventions)
    - [Code Comments](#-code-comments)
    - [Commit Messages](#-commit-messages)
    - [Documentation Pages](#-documentation-pages)
- [Join The Project Team](#-join-the-project-team)

---

## ❓ I Have a Question

> [!NOTE]
> If you want to ask a question, we assume that you have read the available [Documentation](https://gorgeous.simsalabim.studio/docs).

Before you ask a question, it is best to search for existing [Issues](https://github.com/Epic-Nova/Gorgeous-Installer/issues) that might help you. In case you have found a suitable issue and still need clarification, you can write your question in this issue. It is also advisable to search the internet for answers first.

If you then still feel the need to ask a question and need clarification, we recommend the following:

- Read the [FAQ](https://gorgeous.simsalabim.studio/#faq) to see if your question is already answered there.
- Open an [Question Issue](https://github.com/Epic-Nova/Gorgeous-Installer/issues/new?template=question.yaml).
- Write an email to [support@epicnova.net](mailto:support@epicnova.net)

We will then take care of the issue as soon as possible.

---

## 🤝 I Want To Contribute

> [!IMPORTANT]  
> When contributing to this project, **you must agree that you have authored 100% of the content**, that you have the necessary rights to the content and that the content you contribute may be provided under the project license.

### 🐞 Reporting Bugs

#### 🐞 Before Submitting a Bug Report

A good bug report shouldn't leave others needing to chase you up for more information. Therefore, we ask you to investigate carefully, collect information and describe the issue in detail in your report. Please complete the following steps in advance to help us fix any potential bug as fast as possible.

- ✅ **Make sure that you are using the latest version.**
- 🤔 **Determine if your bug is really a bug** and not an error on your side, e.g., using incompatible environment components/versions. (Make sure that you have read the [documentation](https://gorgeous.simsalabim.studio/docs). If you are looking for support, you might want to check [this section](#-i-have-a-question)).
- 🔍 **Check the [bug tracker](https://github.com/Epic-Nova/Gorgeous-Installer/issues)** to see if a bug report already exists for your bug or error.
- 🌐 **Search the internet** (including Stack Overflow) to see if users outside of the GitHub community have discussed the issue.
- Gather information about the bug:
    - 📜 **Stack trace (Traceback)**
    - 💻 **OS, Platform, and Version** (Windows, Linux, macOS, x86, ARM)
    - ⚙️ **Version of the engine, compiler, SDK, runtime environment.**
    - ➡️ **Possibly your input and the output**
    - 🔁 **Can you reliably reproduce the issue?** And can you also reproduce it with older versions?

#### 📝 How Do I Submit a Good Bug Report?

> [!IMPORTANT]
> 🚨 **You must never report security-related issues, vulnerabilities, or bugs including sensitive information to the issue tracker or elsewhere in public.** How to handle vulnerability reporting is described [here](#%EF%B8%8F-reporting-vulnerabilities).

We use GitHub issues to track bugs and errors. If you run into an issue with the project:

- 🐛 **Open an [Issue](https://github.com/Epic-Nova/Gorgeous-Installer/issues/new?template=bug_report.yaml).**
- ✍️ **Explain the behavior you would expect and the actual behavior.**
- ⚙️ **Please provide as much context as possible** and describe the ***reproduction steps*** that someone else can follow to recreate the issue on their own. This usually includes your code. For installer bugs, include the pack type, Unreal Engine version, OS, sanitized logs, and a minimal reproducible payload when possible.
- ℹ️ **Provide the information you collected in the previous section.**

Once it's filed:

- The project team will label the issue accordingly.
- A team member will try to reproduce the issue with your provided steps. If there are no reproduction steps or no obvious way to reproduce the issue, the team will ask you for those steps and mark the issue as $${\color{gold}Needs Update}$$. Bugs with the $${\color{gold}Needs Update}$$ tag will not be addressed until they are reproduced.
- If the team is able to reproduce the issue, then the $${\color{steelblue}Triage}$$ tag will be removed to indicate that this is a valid bug report. Also it will be marked possibly with other tags (such as $${\color{darkblue}Noted}$$), and the issue will be left to be [implemented by someone](#-your-first-code-contribution).

---

### 🛡️ Reporting Vulnerabilities

> [!NOTE]
> Please report vulnerabilities responsibly to help us ensure the safety and security of our community and users.

Vulnerabilities should be reported directly to the project maintainers via a secure and private channel. **Do not disclose security-related issues publicly on the issue tracker or any other public platform.**

To report a security vulnerability, please send an email to: [**vulnerabilities@epicnova.net**](mailto:vulnerabilities@epicnova.net)

To ensure the confidentiality of your report, we strongly encourage you to encrypt your communication using our PGP public key. You can find the public key here:

[🔗 **PGP Public Key**](https://github.com/Epic-Nova/Gorgeous-Core/blob/master/.github/SensitiveReportingPublicKey.asc)

By encrypting your report, you can help protect sensitive information during transmission. We appreciate your cooperation in handling security matters responsibly. Our team will acknowledge your report promptly and work diligently to address the issue.

---

### ✨ Suggesting Enhancements

This section guides you through submitting an enhancement suggestion for Gorgeous Installer, **including completely new features and minor improvements to existing functionality**. Following these guidelines will help maintainers and the community to understand your suggestion and find related suggestions.

#### 🤔 Before Submitting an Enhancement

- ✅ **Make sure that you are using the latest version.**
- 📖 **Read the [documentation](https://gorgeous.simsalabim.studio/docs) carefully** and find out if the functionality is already covered, maybe by an individual configuration.
- 🔍 **Perform a [search](https://github.com/Epic-Nova/Gorgeous-Installer/issues)** to see if the enhancement has already been suggested. If it has, add a comment to the existing issue instead of opening a new one.
- 💡 **Find out whether your idea fits with the scope and aims of the project.** It's up to you to make a strong case to convince the project's developers of the merits of this feature. Keep in mind that we want features that will be useful to the majority of our users and not just a small subset.

#### 📝 How Do I Submit a Good Enhancement Suggestion?

Enhancement suggestions are tracked as [Feature issues](https://github.com/Epic-Nova/Gorgeous-Installer/issues/new?template=feature_request.yaml).

- 🏷️ **Use a clear and descriptive title** for the issue to identify the suggestion.
- 📝 **Provide a step-by-step description of the suggested enhancement** in as many details as possible.
- ➡️ **Describe the current behavior** and **explain which behavior you expected to see instead** and why. At this point you can also tell which alternatives do not work for you.
- 🖼️ **You may want to include screenshots or screen recordings** which help you demonstrate the steps or point out the part which the suggestion is related to. You can use [LICEcap](https://www.cockos.com/licecap/) to record GIFs on macOS and Windows, and the built-in [screen recorder in GNOME](https://help.gnome.org/users/gnome-help/stable/screen-shot-record.html.en) or [SimpleScreenRecorder](https://github.com/MaartenBaert/ssr) on Linux. <!-- this should only be included if the project has a GUI -->
- **Explain why this enhancement would be useful** to most Gorgeous Installer users. You may also want to point out the other projects that solved it better and which could serve as inspiration.

---

### 🧑‍💻 Your First Code Contribution

We appreciate your interest in contributing code to Gorgeous Installer! Here's a quick guide to get you started:

1.  **Fork the Repository:** Click the "Fork" button at the top right of the [GitHub repository](https://github.com/Epic-Nova/Gorgeous-Installer). This creates a copy of the project under your GitHub account.
2.  **Clone Your Fork:** Clone the forked repository to your local machine using Git:
    ```bash
    git clone https://github.com/YOUR_USERNAME/Gorgeous-Installer.git
    cd Gorgeous-Installer
    ```
    Replace `YOUR_USERNAME` with your GitHub username.
3.  **Set Up Your Development Environment:** Follow the instructions in the project's main [`README.md`](https://github.com/Epic-Nova/Gorgeous-Installer/blob/master/README.md) or a dedicated [`DEVELOPMENT.md`](https://github.com/Epic-Nova/Gorgeous-Installer/blob/master/.github/DEVELOPMENT.md) file (if it exists) to set up your development environment. This might include installing dependencies, configuring your IDE, and setting up any necessary SDKs.
4.  **Create a New Branch:** Before making any changes, create a new branch for your contribution:
    ```bash
    git checkout -b feature/your-feature-name
    ```
    or for bug fixes:
    ```bash
    git checkout -b fix/your-bug-fix-name
    ```
    Choose a descriptive name for your branch.
5.  **Make Your Changes:** Implement your feature, bug fix, or improvement. Follow the [Styleguides](#-styleguides) outlined below.
6.  **Test Your Changes:** Thoroughly test your changes to ensure they work as expected and don't introduce any regressions.
7.  **Commit Your Changes:** Commit your changes with a clear and concise commit message following the [Commit Messages](#-commit-messages) styleguide:
    ```bash
    git add .
    git commit -m "Feat: Add new awesome feature"
    ```
    or
    ```bash
    git commit -m "Fix: Resolve issue with widget rendering"
    ```
8.  **Push to Your Fork:** Push your local branch to your remote fork on GitHub:
    ```bash
    git push origin feature/your-feature-name
    ```
9.  **Create a Pull Request:** Go to the main [GitHub repository](https://github.com/Epic-Nova/Gorgeous-Installer) and click the "Compare & pull request" button. Provide a clear and detailed description of your changes in the pull request. Reference any related issues.
10. **Code Review:** Your pull request will be reviewed by the project maintainers. They may provide feedback or ask for changes. Be prepared to address any comments and update your pull request accordingly.
11. **Merging:** Once your pull request is approved, it will be merged into the main branch. Congratulations on your first contribution!

> **Labels on Pull Requests:**
>
> Maintainers will use labels on pull requests to indicate their status and priority. Some labels you might encounter include:
>
> $${\color{darkblue}Noted}$$: Considered important by a contributor, so this request is given a higher priority
> 
> $${\color{steelblue}Triage}$$: Initial review required to determine priority and next steps.
>
> $${\color{forestgreen}Accepted}$$: Accepted by a contributor and is currently in the process of being merged
>
> $${\color{seagreen}Under Review}$$: Under review by a contributor at the moment
>
> $${\color{mediumseagreen}Needs Merge}$$: Preparations to be merged have been completed and it´s now ready to be merged
>
> $${\color{firebrick}Bug Fix}$$: Fixes a bug
>
> $${\color{crimson}Bug Report}$$: Reports a Bug
>
> $${\color{darkorange}Needs Answer from Author}$$: Requires consultation with the author
>
> $${\color{goldenrod}Help Wanted}$$: Attention is needed
>
> $${\color{gold}Needs Update}$$: Needs changes because one or more requirements are not met
>
> $${\color{olive}Optimization}$$: Improves the project's quality or stability
>
> $${\color{teal}New Feature}$$: Not on the roadmap but it´s a good addition and will maybe added to the roadmap
>
> $${\color{darkcyan}Feature}$$: Already on the roadmap and it´s likely to be implemented with some code modifications
>
> $${\color{cadetblue}Feature Request}$$: Community-suggested feature, pending review and roadmap consideration.
>
> $${\color{blue}Documentation}$$: Improvements or additions to the documentation
>
> $${\color{navy}Readme}$$: Improvements or additions in the readme file
>
> $${\color{mediumpurple}Good first Issue}$$: Good for newcomers
>
> $${\color{lavender}Declined}$$: Reviewed and rejected as either not meeting the requirements or contributes nothing to the project
>
> $${\color{thistle}Duplicate}$$: Already exists in itself or in another form
>
> $${\color{lightblue}Invalid}$$: Does not appear to be correct

---

### 📚 Improving The Documentation
We highly value contributions to our documentation! Clear and comprehensive documentation is crucial for the success of Gorgeous Installer. Here's how you can help:

-   **Fix Typos and Grammatical Errors:** If you find any mistakes in the documentation, please submit a pull request with the corrections.
-   **Clarify Existing Content:** If you find any sections of the documentation unclear or confusing, please suggest improvements or rewrite them for better understanding.
-   **Add Missing Information:** If you notice any missing information or topics that should be covered, please contribute the missing content.
-   **Improve Examples:** Providing clear and concise examples can greatly enhance the usability of the documentation.
-   **Translate Documentation:** If you are fluent in other languages, you can help by translating the documentation.

Please ensure that your documentation contributions follow the [Documentation Pages](#-documentation-pages) styleguide. Submit your documentation changes as a pull request, just like code contributions.


---

## 🎨 Styleguides

### 🏷️ Naming Conventions

- Follow Go naming conventions: exported names in PascalCase, unexported names in camelCase.
- Keep package names lowercase and concise; use descriptive filenames.
- Keep installer pack types and JSON configuration keys backward compatible.
- Use clear names for Unreal Engine paths and plugin descriptors.

### 💬 Code Comments

Explain non-obvious behavior, safety constraints, and Unreal Engine compatibility assumptions. Avoid comments that merely repeat the code.

### 📝 Commit Messages

Use concise, imperative commit subjects that describe the change. Keep unrelated refactors separate from fixes.

### 📚 Documentation Pages

Document new pack types with their payload layout, destination, prerequisites, validation rules, and example commands. Include platform-specific behavior and limitations.

---

## 👥 Join The Project Team
We're always excited to welcome passionate and dedicated individuals to the Gorgeous Installer project team! If you're looking to get more deeply involved, contribute on a regular basis, and help shape the future of this project, we encourage you to consider joining us.

**Here are some ways you can express your interest in joining the team:**

* **Consistent Contributions:** Demonstrate your commitment by regularly contributing in areas that interest you, whether it's through code contributions, bug fixes, documentation improvements, or helping to answer community questions.
* **Active Participation:** Engage actively in discussions on GitHub issues, pull requests, and any community forums we might have. Share your insights and ideas.
* **Proactive Identification of Needs:** Show initiative by identifying areas where you can contribute value, whether it's tackling unassigned issues, proposing new features, or improving existing processes.
* **Communication and Collaboration:** Be a good team player by communicating clearly, respectfully, and collaborating effectively with other contributors and maintainers.

**The process for joining the team typically involves the following steps:**

1.  **Express Your Interest:** Reach out to the project maintainers (you can mention your interest in a relevant issue, a pull request, or by sending an email to [support@epicnova.net](mailto:support@epicnova.net) with the subject "Interest in Joining the Team"). Briefly explain your background, areas of interest, and how you'd like to contribute more significantly.
2.  **Demonstrate Your Capabilities:** Continue to contribute consistently and demonstrate your skills and understanding of the project's goals and standards.
3.  **Discussion with the Team:** The existing project team will discuss your interest and contributions. We'll consider your technical skills, communication abilities, and commitment to the project.
4.  **Invitation:** If there's a good fit, you'll receive an invitation to join the team with appropriate responsibilities and access rights.

**Benefits of joining the team:**

* **Direct Impact:** You'll have a more direct impact on the project's direction and development.
* **Deeper Collaboration:** You'll work more closely with core contributors and gain valuable experience.
* **Increased Responsibility:** You may be given more responsibilities, such as reviewing contributions or managing specific areas of the project.
* **Recognition:** Your contributions as a team member will be more prominently recognized.

We value diverse perspectives and are looking for individuals who are passionate about Gorgeous Installer and eager to help it grow. If you think you have what it takes, we'd love to hear from you!

---

## Attribution
This guide is based on the [contributing.md](https://contributing.md/generator)!