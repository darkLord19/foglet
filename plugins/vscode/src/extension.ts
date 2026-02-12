import * as vscode from 'vscode';
import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

interface Worktree {
    name: string;
    path: string;
    branch: string;
    head: string;
    locked: boolean;
    prunable: boolean;
}

export function activate(context: vscode.ExtensionContext) {
    console.log('wtx extension activated');

    // Create tree data provider
    const treeDataProvider = new WorktreeTreeDataProvider();
    
    // Register tree view
    const treeView = vscode.window.createTreeView('wtxWorkspaces', {
        treeDataProvider,
        showCollapseAll: false
    });

    // Register commands
    context.subscriptions.push(
        // Quick switcher
        vscode.commands.registerCommand('wtx.switch', async () => {
            try {
                const worktrees = await listWorktrees();
                
                if (worktrees.length === 0) {
                    vscode.window.showInformationMessage('No worktrees found');
                    return;
                }

                const selected = await vscode.window.showQuickPick(
                    worktrees.map(wt => ({
                        label: wt.name,
                        description: wt.branch,
                        detail: wt.path,
                        worktree: wt
                    })),
                    {
                        placeHolder: 'Select a worktree to open'
                    }
                );

                if (selected) {
                    await openWorktree(selected.worktree.name);
                }
            } catch (error) {
                vscode.window.showErrorMessage(`Error: ${error}`);
            }
        }),

        // List worktrees
        vscode.commands.registerCommand('wtx.list', async () => {
            try {
                const worktrees = await listWorktrees();
                const output = worktrees.map(wt => 
                    `${wt.name} (${wt.branch}) - ${wt.path}`
                ).join('\n');
                
                const doc = await vscode.workspace.openTextDocument({
                    content: output,
                    language: 'plaintext'
                });
                
                await vscode.window.showTextDocument(doc);
            } catch (error) {
                vscode.window.showErrorMessage(`Error: ${error}`);
            }
        }),

        // Create worktree
        vscode.commands.registerCommand('wtx.create', async () => {
            try {
                const name = await vscode.window.showInputBox({
                    prompt: 'Enter worktree name',
                    placeHolder: 'feature-branch'
                });

                if (!name) {
                    return;
                }

                const branch = await vscode.window.showInputBox({
                    prompt: 'Enter branch name (leave empty to use worktree name)',
                    placeHolder: name
                });

                await createWorktree(name, branch || name);
                vscode.window.showInformationMessage(`Worktree '${name}' created`);
                treeDataProvider.refresh();
            } catch (error) {
                vscode.window.showErrorMessage(`Error: ${error}`);
            }
        }),

        // Delete worktree
        vscode.commands.registerCommand('wtx.delete', async (item?: WorktreeItem) => {
            try {
                let name: string | undefined;
                
                if (item) {
                    name = item.worktree.name;
                } else {
                    const worktrees = await listWorktrees();
                    const selected = await vscode.window.showQuickPick(
                        worktrees.map(wt => ({
                            label: wt.name,
                            worktree: wt
                        })),
                        { placeHolder: 'Select worktree to delete' }
                    );
                    
                    if (selected) {
                        name = selected.worktree.name;
                    }
                }

                if (!name) {
                    return;
                }

                const confirm = await vscode.window.showWarningMessage(
                    `Delete worktree '${name}'?`,
                    { modal: true },
                    'Delete'
                );

                if (confirm === 'Delete') {
                    await deleteWorktree(name);
                    vscode.window.showInformationMessage(`Worktree '${name}' deleted`);
                    treeDataProvider.refresh();
                }
            } catch (error) {
                vscode.window.showErrorMessage(`Error: ${error}`);
            }
        }),

        // Refresh
        vscode.commands.registerCommand('wtx.refresh', () => {
            treeDataProvider.refresh();
        }),

        treeView
    );

    // Auto-refresh when window gains focus
    vscode.window.onDidChangeWindowState(e => {
        if (e.focused) {
            treeDataProvider.refresh();
        }
    });
}

export function deactivate() {}

// Helper functions

async function listWorktrees(): Promise<Worktree[]> {
    try {
        const { stdout } = await execAsync('wtx list --json');
        return JSON.parse(stdout);
    } catch (error) {
        throw new Error(`Failed to list worktrees: ${error}`);
    }
}

async function openWorktree(name: string): Promise<void> {
    const { stdout } = await execAsync(`wtx open ${name} --editor vscode`);
}

async function createWorktree(name: string, branch: string): Promise<void> {
    await execAsync(`wtx add ${name} ${branch}`);
}

async function deleteWorktree(name: string): Promise<void> {
    await execAsync(`wtx rm ${name}`);
}

// Tree view provider

class WorktreeTreeDataProvider implements vscode.TreeDataProvider<WorktreeItem> {
    private _onDidChangeTreeData: vscode.EventEmitter<WorktreeItem | undefined | null | void> = 
        new vscode.EventEmitter<WorktreeItem | undefined | null | void>();
    
    readonly onDidChangeTreeData: vscode.Event<WorktreeItem | undefined | null | void> = 
        this._onDidChangeTreeData.event;

    refresh(): void {
        this._onDidChangeTreeData.fire();
    }

    getTreeItem(element: WorktreeItem): vscode.TreeItem {
        return element;
    }

    async getChildren(element?: WorktreeItem): Promise<WorktreeItem[]> {
        if (element) {
            return [];
        }

        try {
            const worktrees = await listWorktrees();
            return worktrees.map(wt => new WorktreeItem(wt));
        } catch (error) {
            vscode.window.showErrorMessage(`Error loading worktrees: ${error}`);
            return [];
        }
    }
}

class WorktreeItem extends vscode.TreeItem {
    constructor(public readonly worktree: Worktree) {
        super(worktree.name, vscode.TreeItemCollapsibleState.None);
        
        this.description = worktree.branch;
        this.tooltip = worktree.path;
        this.contextValue = 'worktree';
        
        this.command = {
            command: 'wtx.switch',
            title: 'Open Worktree',
            arguments: []
        };
    }
}
