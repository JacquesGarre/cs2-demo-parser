import { CommonModule, DecimalPipe } from '@angular/common';
import { Component, inject, signal } from '@angular/core';
import { Router } from '@angular/router';
import { HttpErrorResponse } from '@angular/common/http';
import { SubmitDemoUseCase } from '../../core/application/use-cases/submit-demo.use-case';

@Component({
  selector: 'app-upload-page',
  imports: [CommonModule, DecimalPipe],
  templateUrl: './upload-page.component.html',
  styleUrl: './upload-page.component.scss',
})
export class UploadPageComponent {
  private readonly submitDemo = inject(SubmitDemoUseCase);
  private readonly router = inject(Router);

  readonly selectedFileName = signal<string>('');
  readonly isUploading = signal(false);
  readonly errorMessage = signal('');
  readonly uploadProgress = signal(0);
  private progressInterval?: ReturnType<typeof setInterval>;

  onFileSelected(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];

    if (!file) {
      return;
    }

    this.selectedFileName.set(file.name);
    this.errorMessage.set('');
    this.uploadFile(file);
  }

  private uploadFile(file: File): void {
    this.isUploading.set(true);
    this.uploadProgress.set(5);

    this.progressInterval = setInterval(() => {
      const current = this.uploadProgress();
      if (current < 95) {
        this.uploadProgress.update(p => Math.min(95, p + (Math.random() * 5 + 2)));
      }
    }, 600);

    this.submitDemo.execute(file).subscribe({
      next: (job) => {
        clearInterval(this.progressInterval);
        this.uploadProgress.set(100);
        setTimeout(() => {
          this.isUploading.set(false);
          this.router.navigate(['/dashboard'], {
            queryParams: { jobId: job.id, demoId: job.demoId },
          });
        }, 250);
      },
      error: (error: HttpErrorResponse) => {
        clearInterval(this.progressInterval);
        this.isUploading.set(false);
        const backendError = error.error?.error;
        this.errorMessage.set(backendError || 'Upload failed. Please check the .dem file and try again.');
      },
    });
  }
}
