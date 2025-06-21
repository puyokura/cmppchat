// main.cppの初期内容をここに記述します。UIとバックエンドの調整は後ほど行います。
#include <iostream>
#include <string>
#include <limits>

int main() {
    std::cout << "CppChat へようこそ！\n";
    std::cout << "終了するには 'exit' と入力してください。\n";

    std::string user_input;

    while (true) {
        std::cout << "あなた: ";
        std::getline(std::cin, user_input);

        if (user_input == "exit") {
            std::cout << "CppChat を終了します。\n";
            break;
        }

        if (user_input.empty()) {
            std::cout << "CppChat: 何か入力してください。\n";
        } else {
            std::cout << "CppChat: 「" << user_input << "」と入力しましたね。\n";
        }
    }

    return 0;
}