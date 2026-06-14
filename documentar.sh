#!/bin/bash

# Nome do arquivo de saída
OUTPUT_FILE="estrutura_projeto.txt"

# Limpa o arquivo de saída se ele já existir
echo "Gerando documentação do projeto..." > "$OUTPUT_FILE"
echo "Gerado em: $(date)" >> "$OUTPUT_FILE"
echo -e "=========================================\n" >> "$OUTPUT_FILE"

# 1. Mapeia a Arquitetura do Projeto
echo "### ARQUITETURA DO PROJETO ###" >> "$OUTPUT_FILE"
echo "-----------------------------------------" >> "$OUTPUT_FILE"

# Tenta usar o 'tree' se estiver disponível, senão usa o 'find' formatado
if command -v tree &> /dev/null; then
    # Ignora pastas comuns de build, dependências ou ambiente
    tree -I ".git|vendor|.wsl|bin" >> "$OUTPUT_FILE"
else
    find . -not -path '*/.*' -not -path './vendor*' -not -path './bin*' | sed -e 's/[^- \/[a-zA-Z0-9_]]/|/g' -e 's/|/  /g' >> "$OUTPUT_FILE"
fi

echo -e "\n=========================================\n" >> "$OUTPUT_FILE"

# 2. Varre e adiciona o conteúdo dos arquivos Go (.go, go.mod, go.sum)
echo "### CONTEÚDO DOS ARQUIVOS ###" >> "$OUTPUT_FILE"
echo -e "-----------------------------------------\n" >> "$OUTPUT_FILE"

# Localiza arquivos específicos do ecossistema Go e configurações essenciais
find . -type f \( -name "*.go" -o -name "go.mod" -o -name "go.sum" \) \
    -not -path '*/.*' \
    -not -path './vendor/*' \
    -not -path './bin/*' | while read -r arquivo; do
    
    echo "[$arquivo]" >> "$OUTPUT_FILE"
    echo "-----------------------------------------" >> "$OUTPUT_FILE"
    cat "$arquivo" >> "$OUTPUT_FILE"
    echo -e "\n\n" >> "$OUTPUT_FILE"
done

echo "Pronto! O relatório foi gerado em: $OUTPUT_FILE"